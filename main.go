package main

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

const (
	// Test için ayarları projenin kök dizininden okuyacağız.
	configPath   = "config.json"
	pollInterval = 60 * time.Second
)

func main() {
	log.Println("--- EdgeSync Agent Başlatılıyor ---")

	// 1. Yapılandırmayı Yükle
	cfg, err := LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Yapılandırma dosyası '%s' okunamadı: %v", configPath, err)
	}
	log.Printf("Yapılandırma yüklendi: %+v", *cfg)

	// 2. Gerçek Bileşenleri Oluştur
	s3Client, err := NewRealS3Client()
	if err != nil {
		log.Fatalf("S3 istemcisi oluşturulamadı: %v", err)
	}

	deployer := &RealDeployer{}
	linker := &RealLinker{}

	// 3. Poller'ı Oluştur
	// Poller, tüm bağımlılıkları (config, s3, deployer, linker) alarak oluşturulur.
	poller := NewPoller(cfg, s3Client, deployer, linker, "active_model_link")

	log.Printf("Poller başarıyla oluşturuldu. İlk kontrol %v sonra başlayacak.", pollInterval)

	// 4. Arka Plan Poller'ını Başlat
	go func() {
		for {
			log.Println("[Poller Worker] Yeni model kontrol ediliyor...")
			if err := poller.RunOnce(); err != nil {
				log.Printf("[Poller Worker] Hata: %v", err)
			}
			time.Sleep(pollInterval)
		}
	}()

	// 5. HTTP Sunucusunu Başlat (Durum ve Sağlık Kontrolü için)
	// Bu, ana goroutine'in sonlanmasını engeller.
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		currentETag := poller.GetStatus()
		fmt.Fprintf(w, "<h1>EdgeSync Agent Status</h1><p>Current Active Model ETag: %s</p>", currentETag)
	})

	log.Println("Web sunucusu http://localhost:8080 adresinde başlatılıyor...")
	log.Fatal(http.ListenAndServe("localhost:8080", nil))
}
