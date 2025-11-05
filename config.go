package main // Bu dosyanın, bir kütüphane değil, çalıştırılabilir bir program olduğunu belirtir.

import (
	"encoding/json" // JSON verilerini okumak ve yazmak için (Adım 3)
	"os"            // İşletim sistemi fonksiyonları için, örneğin dosya okuma (Adım 3)
)

// Config (Yapı), bizim JSON yapılandırma dosyamızın Go dilindeki temsilcisidir.
// 'json:"..."' etiketleri (tags), Go'daki (büyük harfli) alan adını
// JSON dosyasındaki (küçük harfli) karşılığına eşler.
type Config struct {
	S3Bucket         string `json:"s3_bucket"`
	S3Key            string `json:"s3_key"`
	DeployScriptPath string `json:"deploy_script_path"`
}

// LoadConfig, belirtilen yoldan (path) bir JSON yapılandırma dosyası okur
// ve bunu bir Config struct'ına dönüştürür.
func LoadConfig(path string) (*Config, error) {
	// 1. Adım: Dosyayı diskten oku.
	// os.ReadFile, dosyanın tüm içeriğini bir byte dizisi (data) olarak döndürür.
	data, err := os.ReadFile(path)
	if err != nil {
		// Eğer dosya okunamadıysa (örn: dosya yok), hatayı döndür.
		return nil, err
	}

	// 2. Adım: Boş bir Config struct'ı oluştur.
	var cfg Config

	// 3. Adım: JSON verisini (data) Go struct'ına (cfg) "Unmarshal" et (dönüştür).
	// json.Unmarshal, JSON verisini alır ve 'json' etiketlerine bakarak
	// veriyi cfg değişkeninin içine doldurur.
	err = json.Unmarshal(data, &cfg)
	if err != nil {
		// Eğer JSON formatı bozuksa, hatayı döndür.
		return nil, err
	}

	// 4. Adım: Her şey başarılıysa, yapılandırmayı (cfg) ve nil (hata yok) döndür.
	return &cfg, nil
}
