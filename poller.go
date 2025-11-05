package main

import (
	"fmt"
	"log" // Ekrana/dosyaya log basmak için
	"sync"
)

// --- ARAYÜZLER (INTERFACES) ---
// Bu arayüzler, "sahte" (mock) ve "gerçek" bileşenler arasında
// bir sözleşme görevi görür. Poller'ımız sadece bu arayüzlerle konuşur.

// S3Client, S3'ten bilgi almak ve indirmek için gereken fonksiyonları tanımlar.
type S3Client interface {
	// HeadObject, bir dosyanın ETag'ini (versiyonunu) döndürür.
	HeadObject(bucket, key string) (string, error)
	// DownloadObject, bir dosyayı S3'ten indirir.
	DownloadObject(bucket, key, destinationPath string) error
}

// Deployer, bir script'i çalıştırmak için gereken fonksiyonu tanımlar.
type Deployer interface {
	// Run, belirtilen script'i verilen argümanlarla çalıştırır.
	// Dönen 'error', script'in 'exit code 0' dışında bir kodla bitmesi durumudur.
	Run(scriptPath string, args ...string) error
}

// Linker, sembolik bağ (symlink) yönetimi için gereken fonksiyonu tanımlar.
type Linker interface {
	// Set, 'linkName' (kısayol) 'target' (hedef dosya) 'i gösterecek şekilde ayarlar.
	Set(target, linkName string) error
	// Get, 'linkName' (kısayol) 'in şu an nereyi gösterdiğini döndürür.
	Get(linkName string) (string, error)
}

// --- ÇEKİRDEK YAPI (POLLER STRUCT) ---

// Poller, ajanımızın tüm durumunu (state) ve bağımlılıklarını (dependencies) tutar.
type Poller struct {
	// Bağımlılıklar (Dış Dünya)
	s3     S3Client
	deploy Deployer
	linker Linker

	// Yapılandırma (Config'den gelen)
	cfg *Config

	// Durum (State)
	// mu, lastKnownETag gibi state alanlarına eşzamanlı erişimi korur.
	mu              sync.RWMutex
	lastKnownETag   string // En son başarıyla deploy edilen modelin ETag'i
	activeModelPath string // Sembolik bağın (link) adı
}

// NewPoller, yeni bir Poller struct'ı oluşturmak için "constructor" fonksiyonudur.
func NewPoller(cfg *Config, s3 S3Client, deploy Deployer, linker Linker, activePath string) *Poller {
	return &Poller{
		s3:              s3,
		deploy:          deploy,
		linker:          linker,
		cfg:             cfg,
		activeModelPath: activePath,
		// lastKnownETag başlangıçta boştur, ilk çalışmada set edilecek.
	}
}

// RunOnce, Poller'ın bir kontrol döngüsünü çalıştırır (Akış B).
func (p *Poller) RunOnce() error {
	log.Println("[Poller] Yeni model versiyonu kontrol ediliyor...")

	// 1. ADIM: S3'ü Kontrol Et (FG3)
	remoteETag, err := p.s3.HeadObject(p.cfg.S3Bucket, p.cfg.S3Key)
	if err != nil {
		return fmt.Errorf("S3 HeadObject hatası: %w", err)
	}

	// 2. ADIM: ETag'leri Karşılaştır
	if p.lastKnownETag == "" {
		// Bu, ajanın ilk çalışması. Mevcut ETag'i "bilinen" olarak kaydet.
		log.Printf("[Poller] İlk çalışma. Mevcut ETag '%s' olarak ayarlandı.", remoteETag)
		p.mu.Lock()
		p.lastKnownETag = remoteETag
		p.mu.Unlock()
		// Ve mevcut sembolik bağın hedefini al (eğer varsa)
		// p.activeModelPath'in nereyi gösterdiğini öğrenip onu "eski" model olarak saklayabiliriz.
		// Şimdilik basitleştirelim: ilk çalışmada deploy yapma, sadece durumu öğren.
		return nil
	}

	if remoteETag == p.lastKnownETag {
		// Değişiklik yok.
		log.Println("[Poller] Model değişmemiş. (ETag:", remoteETag, ")")
		return nil
	}

	// 3. ADIM: YENİ MODEL VAR! (FG4)
	log.Printf("[Poller] YENİ MODEL ALGILANDI! Eski: '%s', Yeni: '%s'", p.lastKnownETag, remoteETag)

	// Eski (mevcut) çalışan modeli bul (Rollback için lazım)
	// Tasarımda `active_model` adını /var/lib/edgesync/active_model olarak belirlemiştik
	oldModelTarget, err := p.linker.Get(p.activeModelPath)
	if err != nil {
		log.Printf("[Poller] UYARI: Rollback için eski modelin yolu okunamadı: %v", err)
		// oldModelTarget="" olarak devam et, bu durumda rollback yapılamaz.
	}

	// Yeni modeli indirilecek geçici bir yer belirle
	// Tasarım: /var/lib/edgesync/staging/model-[ETag].bin
	// Şimdilik test için basit bir yol kullanalım (MockLinker bunu önemsemeyecek)
	newModelDownloadPath := fmt.Sprintf("models/model-%s.bin", remoteETag)

	err = p.s3.DownloadObject(p.cfg.S3Bucket, p.cfg.S3Key, newModelDownloadPath)
	if err != nil {
		return fmt.Errorf("S3 DownloadObject hatası: %w", err)
	}
	log.Printf("[Poller] Yeni model '%s' adresine başarıyla indirildi.", newModelDownloadPath)

	// 4. ADIM: Test Et (FG5c)
	log.Println("[Poller] Yeni model test ediliyor... (`deploy.sh --test`)")
	err = p.deploy.Run(p.cfg.DeployScriptPath, "--test", newModelDownloadPath)
	if err != nil {
		// Test başarısız! Dağıtımı iptal et.
		return fmt.Errorf("yeni model testi BAŞARISIZ oldu: %w", err)
	}
	log.Println("[Poller] Yeni model testi BAŞARILI.")

	// 5. ADIM: Atomik Değişim (Symlink) (FG5d)
	log.Printf("[Poller] Sembolik bağ (symlink) '%s' -> '%s' olarak değiştiriliyor...", p.activeModelPath, newModelDownloadPath)
	err = p.linker.Set(newModelDownloadPath, p.activeModelPath)
	if err != nil {
		return fmt.Errorf("sembolik bağ değiştirilemedi: %w", err)
	}

	// 6. ADIM: Servisi Yeniden Başlat (FG5e)
	log.Println("[Poller] Servis yeniden başlatılıyor... (`deploy.sh --reload`)")
	err = p.deploy.Run(p.cfg.DeployScriptPath, "--reload")
	if err != nil {
		// YENİDEN BAŞLATMA BAŞARISIZ! OTOMATİK ROLLBACK (FG6.3)
		log.Printf("[Poller] HATA! Servis yeni modelle başlatılamadı: %v", err)
		log.Println("[Poller] OTOMATİK ROLLBACK BAŞLATILIYOR...")

		if oldModelTarget == "" {
			return fmt.Errorf("ROLLBACK BAŞARISIZ: Eski modelin yolu bilinmiyor")
		}

		// Sembolik bağı acilen ESKİ modele geri çevir.
		errRollback := p.linker.Set(oldModelTarget, p.activeModelPath)
		if errRollback != nil {
			// Bu olursa çok büyük felaket (sistem "down" kalır)
			return fmt.Errorf("KRİTİK HATA! Rollback sırasında sembolik bağ değiştirilemedi: %w", errRollback)
		}

		// Servisi ESKİ modelle tekrar başlat.
		errReloadOld := p.deploy.Run(p.cfg.DeployScriptPath, "--reload")
		if errReloadOld != nil {
			return fmt.Errorf("KRİTİK HATA! Rollback başarılı ancak servis eski modelle de başlatılamadı: %w", errReloadOld)
		}

		log.Println("[Poller] ROLLBACK BAŞARILI. Sistem eski stabil modele döndü.")
		// Orijinal hatayı döndür ki loglarda görünsün.
		return fmt.Errorf("dağıtım hatası (rollback yapıldı): %w", err)
	}

	// 7. ADIM: BAŞARILI!
	log.Printf("[Poller] DAĞITIM BAŞARILI. Yeni aktif model ETag: '%s'", remoteETag)
	p.mu.Lock()
	p.lastKnownETag = remoteETag // Durumu güncelle.
	p.mu.Unlock()

	return nil
}

// GetStatus, Poller'ın mevcut bilinen ETag'ini thread-safe bir şekilde döndürür.
func (p *Poller) GetStatus() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.lastKnownETag
}
