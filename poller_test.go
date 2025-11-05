package main

import (
	"fmt"     // Hata oluşturmak için
	"strings" // Çağrıları kaydetmek için
	"testing" // Test kütüphanesi
)

// --- SAHTE BİLEŞENLER (MOCKS) ---
// Bu yapılar, poller.go'daki arayüzleri (interface) taklit eder.
// Gerçek S3'e veya diske dokunmazlar. Sadece "çağrıldık mı?" diye kaydederler.

// MockS3Client, S3Client arayüzünü taklit eder.
type MockS3Client struct {
	// Bu alanları testin içinde biz ayarlayacağız.
	// Poller bu fonksiyonu çağırdığında, bu değerleri döndürecek.
	EtagToReturn string
	ErrToReturn  error
}

func (m *MockS3Client) HeadObject(bucket, key string) (string, error) {
	return m.EtagToReturn, m.ErrToReturn
}

func (m *MockS3Client) DownloadObject(bucket, key, destinationPath string) error {
	return m.ErrToReturn
}

// MockDeployer, Deployer arayüzünü taklit eder.
type MockDeployer struct {
	// Hangi argümanlarla hata vereceğini test içinde belirtebiliriz.
	// Örn: "--reload" argümanı gelirse hata ver.
	FailOnArgs []string
	// Yaptığı tüm çağrıları kaydeder (testin sonunda kontrol etmek için)
	Calls []string
}

func (m *MockDeployer) Run(scriptPath string, args ...string) error {
	// Çağrıyı kaydet. Örn: "deploy.sh --test /var/lib/model.bin"
	call := fmt.Sprintf("%s %s", scriptPath, strings.Join(args, " "))
	m.Calls = append(m.Calls, call)

	// Hata verme koşulunu kontrol et
	if len(m.FailOnArgs) > 0 {
		for _, argToFail := range m.FailOnArgs {
			for _, arg := range args {
				if arg == argToFail {
					m.FailOnArgs = nil // Hata sadece bir kez tetiklenir.
					return fmt.Errorf("MockDeployer: '%s' argümanı için hata vermeye ayarlandı", arg)
				}
			}
		}
	}
	return nil // Başarılı
}

// MockLinker, Linker arayüzünü taklit eder.
type MockLinker struct {
	CurrentTarget string // Şu anki hedefi (eski model)
	Calls         []string
}

func (m *MockLinker) Set(target, linkName string) error {
	call := fmt.Sprintf("SET %s -> %s", linkName, target)
	m.Calls = append(m.Calls, call)
	m.CurrentTarget = target // Durumu güncelle
	return nil
}

func (m *MockLinker) Get(linkName string) (string, error) {
	return m.CurrentTarget, nil
}

// --- TEST FONKSİYONLARI (BAŞARI KRİTERİ) ---

// TestPoller_HappyPath (Mutlu Son Senaryosu)
// S3'te yeni model var, test başarılı, deploy başarılı.
func TestPoller_HappyPath(t *testing.T) {
	// 1. Hazırlık (Setup)
	mockCfg := &Config{
		S3Bucket:         "test-bucket",
		S3Key:            "model.bin",
		DeployScriptPath: "deploy.sh",
	}
	mockS3 := &MockS3Client{EtagToReturn: "v2-new-model"}                  // S3 "v2" döndürecek
	mockDeploy := &MockDeployer{}                                          // Deployer hata vermeyecek
	mockLink := &MockLinker{CurrentTarget: "/var/lib/models/model-v1.bin"} // Mevcut model "v1"

	activeModelLink := "/var/lib/edgesync/active_model"

	p := NewPoller(mockCfg, mockS3, mockDeploy, mockLink, activeModelLink)
	p.lastKnownETag = "v1-old-model" // Ajan "v1" i biliyor

	// 2. Çalıştırma (Execute)
	err := p.RunOnce()

	// 3. Doğrulama (Assert)
	if err != nil {
		t.Fatalf("RunOnce() beklenmedik bir hata döndürdü: %v", err)
	}

	// Deployer doğru çağrıldı mı?
	// Önce test, sonra reload
	if len(mockDeploy.Calls) != 2 {
		t.Fatalf("Deployer.Run() 2 kez çağrılmalıydı, ancak %d kez çağrıldı: %v", len(mockDeploy.Calls), mockDeploy.Calls)
	}

	// YENİ KATILAŞTIRMA:
	// Sadece '--test' içeriyor mu diye bakma, TAM OLARAK ne çağırması gerektiğine bak.
	expectedCall0 := "deploy.sh --test /var/lib/edgesync/models/model-v2-new-model.bin"
	if mockDeploy.Calls[0] != expectedCall0 {
		t.Errorf("İlk deploy çağrısı hatalı.\nBeklenen: '%s'\nAlınan:   '%s'", expectedCall0, mockDeploy.Calls[0])
	}

	expectedCall1 := "deploy.sh --reload"
	if mockDeploy.Calls[1] != expectedCall1 {
		t.Errorf("İkinci deploy çağrısı hatalı.\nBeklenen: '%s'\nAlınan:   '%s'", expectedCall1, mockDeploy.Calls[1])
	}

	// Linker doğru çağrıldı mı?
	if len(mockLink.Calls) != 1 {
		t.Fatalf("Linker.Set() 1 kez çağrılmalıydı, ancak %d kez çağrıldı", len(mockLink.Calls))
	}
	expectedLinkTarget := "/var/lib/edgesync/models/model-v2-new-model.bin"
	if mockLink.CurrentTarget != expectedLinkTarget {
		t.Errorf("Linker hedefi '%s' olmalıydı, ancak '%s' oldu", expectedLinkTarget, mockLink.CurrentTarget)
	}

	// Poller durumu güncellendi mi?
	if p.lastKnownETag != "v2-new-model" {
		t.Errorf("Poller'ın son ETag'i 'v2-new-model' olmalıydı, ancak '%s' oldu", p.lastKnownETag)
	}
}

// TestPoller_RollbackPath (Hata ve Geri Alma Senaryosu)
// S3'te yeni model var, test başarılı, AMA deploy (--reload) başarısız.
func TestPoller_RollbackPath(t *testing.T) {
	// 1. Hazırlık (Setup)
	mockCfg := &Config{
		S3Bucket:         "test-bucket",
		S3Key:            "model.bin",
		DeployScriptPath: "deploy.sh",
	}
	mockS3 := &MockS3Client{EtagToReturn: "v2-new-model"} // S3 "v2" döndürecek
	mockDeploy := &MockDeployer{
		FailOnArgs: []string{"--reload"}, // "--reload" argümanını görünce HATA VER.
	}

	eskiModelYolu := "/var/lib/models/model-v1.bin"
	mockLink := &MockLinker{CurrentTarget: eskiModelYolu} // Mevcut model "v1"

	activeModelLink := "/var/lib/edgesync/active_model"

	p := NewPoller(mockCfg, mockS3, mockDeploy, mockLink, activeModelLink)
	p.lastKnownETag = "v1-old-model" // Ajan "v1" i biliyor

	// 2. Çalıştırma (Execute)
	err := p.RunOnce()

	// 3. Doğrulama (Assert)
	if err == nil {
		t.Fatal("RunOnce() hata döndürmeliydi (Rollback), ancak nil döndürdü.")
	}
	if !strings.Contains(err.Error(), "rollback yapıldı") {
		t.Errorf("Hata mesajı 'rollback yapıldı' içermeliydi: %v", err)
	}

	// Deployer doğru çağrıldı mı?
	// 1. --test (Başarılı)
	// 2. --reload (Başarısız)
	// 3. --reload (Rollback için tekrar - Başarılı)
	if len(mockDeploy.Calls) != 3 {
		t.Fatalf("Deployer.Run() 3 kez çağrılmalıydı (test, fail, rollback), ancak %d kez çağrıldı: %v", len(mockDeploy.Calls), mockDeploy.Calls)
	}

	// Linker doğru çağrıldı mı?
	// 1. Yeni modele set et (SET ... -> ...v2)
	// 2. Eski modele geri set et (SET ... -> ...v1)
	if len(mockLink.Calls) != 2 {
		t.Fatalf("Linker.Set() 2 kez çağrılmalıydı (set, rollback), ancak %d kez çağrıldı", len(mockLink.Calls))
	}
	if mockLink.CurrentTarget != eskiModelYolu {
		t.Errorf("Rollback sonrası Linker hedefi ESKİ model ('%s') olmalıydı, ancak '%s' oldu", eskiModelYolu, mockLink.CurrentTarget)
	}

	// Poller durumu GÜNCELLENMEMELİ!
	if p.lastKnownETag != "v1-old-model" {
		t.Errorf("Poller'ın son ETag'i değişmemeliydi ('v1-old-model'), ancak '%s' oldu", p.lastKnownETag)
	}
}

// TestPoller_NoChange (Değişiklik Yok Senaryosu)
// S3'teki ETag, bilinen ETag ile aynı.
func TestPoller_NoChange(t *testing.T) {
	// 1. Hazırlık (Setup)
	mockCfg := &Config{DeployScriptPath: "deploy.sh"}
	mockS3 := &MockS3Client{EtagToReturn: "v1-model"} // S3 "v1" döndürecek
	mockDeploy := &MockDeployer{}
	mockLink := &MockLinker{}

	p := NewPoller(mockCfg, mockS3, mockDeploy, mockLink, "")
	p.lastKnownETag = "v1-model" // Ajan zaten "v1" i biliyor

	// 2. Çalıştırma (Execute)
	err := p.RunOnce()

	// 3. Doğrulama (Assert)
	if err != nil {
		t.Fatalf("RunOnce() beklenmedik bir hata döndürdü: %v", err)
	}
	// Hiçbir şey çağrılmamalı!
	if len(mockDeploy.Calls) != 0 {
		t.Errorf("Deployer.Run() çağrılmamalıydı, ancak %d kez çağrıldı", len(mockDeploy.Calls))
	}
	if len(mockLink.Calls) != 0 {
		t.Errorf("Linker.Set() çağrılmamalıydı, ancak %d kez çağrıldı", len(mockLink.Calls))
	}
}
