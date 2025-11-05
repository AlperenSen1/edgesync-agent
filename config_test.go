package main // Testler de ana paketimizin bir parçasıdır.

import (
	"os"      // Test için sahte dosya oluşturmak/silmek için
	"testing" // Go'nun test kütüphanesi
)

// TestLoadConfig, bizim LoadConfig fonksiyonumuzun doğru çalışıp çalışmadığını test eder.
func TestLoadConfig(t *testing.T) {
	// 1. Adım: Sahte bir JSON yapılandırması hazırla.
	// (Not: Go'da "raw string" için ` karakteri kullanılır, \ kaçış karakterine gerek kalmaz)
	sahteConfigJSON := `{
		"s3_bucket": "benim-test-bucketim",
		"s3_key": "modeller/model.bin",
		"deploy_script_path": "/test/deploy.sh"
	}`

	// 2. Adım: Test için geçici bir dosya oluştur ve sahte JSON'u içine yaz.
	tmpFile, err := os.CreateTemp("", "test_config_*.json")
	if err != nil {
		t.Fatalf("Geçici test dosyası oluşturulamadı: %v", err)
	}

	// 3. Adım: t.Cleanup(), "bu test bittiği an (başarılı da olsa, hata da verse)
	// bu fonksiyonu çalıştır" demektir. Bu, testimizin çöp bırakmamasını sağlar.
	t.Cleanup(func() {
		os.Remove(tmpFile.Name()) // Test bitince sahte dosyayı sil.
	})

	// Sahte veriyi geçici dosyaya yaz.
	if _, err := tmpFile.Write([]byte(sahteConfigJSON)); err != nil {
		t.Fatalf("Geçici dosyaya yazılamadı: %v", err)
	}
	tmpFile.Close() // Dosyayı yazdıktan sonra kapat.

	// 4. Adım: Test Etme (Asıl İş)
	// Bizim yazdığımız LoadConfig fonksiyonunu, bu sahte dosyanın yoluyla çağır.
	cfg, err := LoadConfig(tmpFile.Name())

	// 5. Adım: Sonuçları Doğrulama (Assert)
	if err != nil {
		t.Fatalf("LoadConfig fonksiyonu beklenmedik bir hata döndürdü: %v", err)
	}

	if cfg == nil {
		t.Fatalf("Config (cfg) nil (boş) döndü, ancak bir struct bekleniyordu.")
	}

	// En önemli kontrol: Dosyadaki "s3_bucket" değeri, struct'a doğru gelmiş mi?
	beklenenBucket := "benim-test-bucketim"
	if cfg.S3Bucket != beklenenBucket {
		t.Errorf("S3Bucket için beklenen değer '%s', ancak alınan değer '%s'", beklenenBucket, cfg.S3Bucket)
	}

	// Diğer alanları da kontrol edelim.
	beklenenKey := "modeller/model.bin"
	if cfg.S3Key != beklenenKey {
		t.Errorf("S3Key için beklenen değer '%s', ancak alınan değer '%s'", beklenenKey, cfg.S3Key)
	}
}
