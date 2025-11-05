package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// RealS3Client, S3Client arayüzünün AWS SDK kullanarak gerçek bir
// S3 servisiyle konuşan implementasyonudur.
type RealS3Client struct {
	client     *s3.Client
	downloader *manager.Downloader
}

// NewRealS3Client, varsayılan AWS kimlik bilgilerini (ortam değişkenleri, IAM rolü vb.)
// kullanarak yeni bir RealS3Client oluşturur.
func NewRealS3Client() (*RealS3Client, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, fmt.Errorf("aws config yüklenemedi: %w", err)
	}

	s3Client := s3.NewFromConfig(cfg)
	downloader := manager.NewDownloader(s3Client)

	return &RealS3Client{
		client:     s3Client,
		downloader: downloader,
	}, nil
}

// RealLinker, Linker arayüzünün os paketini kullanarak gerçek sembolik bağları
// yöneten implementasyonudur.
type RealLinker struct{}

// Set, hedefi gösteren bir sembolik bağ oluşturur.
// Eğer o isimde bir dosya/link zaten varsa, önce onu siler.
func (rl *RealLinker) Set(target, linkName string) error {
	// Önce mevcut linki (varsa) kaldır.
	err := os.Remove(linkName)
	// Eğer dosya zaten yoksa bu bir hata değildir, devam et.
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.Symlink(target, linkName)
}

// Get, bir sembolik bağın hedefini okur.
func (rl *RealLinker) Get(linkName string) (string, error) {
	return os.Readlink(linkName)
}

// RealDeployer, Deployer arayüzünün os/exec kullanarak gerçek script'leri
// çalıştıran implementasyonudur.
type RealDeployer struct{}

// Run, belirtilen script'i argümanlarıyla çalıştırır.
// Script'in 'exit code 0' dışında bir kodla bitmesi durumunda,
// hem hatayı hem de script'in çıktısını içeren bir error döndürür.
func (rd *RealDeployer) Run(scriptPath string, args ...string) error {
	cmd := exec.Command(scriptPath, args...)
	output, err := cmd.CombinedOutput() // stdout ve stderr'i birleştirir.

	if err != nil {
		return fmt.Errorf("script '%s %s' hatayla sonlandı: %w. Çıktı: %s", scriptPath, strings.Join(args, " "), err, string(output))
	}

	return nil
}

// HeadObject, S3'teki bir nesnenin metadata'sını (özellikle ETag) almak için
// AWS SDK'sını kullanır.
func (r *RealS3Client) HeadObject(bucket, key string) (string, error) {
	output, err := r.client.HeadObject(context.TODO(), &s3.HeadObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err != nil {
		return "", fmt.Errorf("S3 HeadObject (%s/%s) hatası: %w", bucket, key, err)
	}

	// S3 ETag'leri genellikle çift tırnak içinde gelir ("..."), bunları temizliyoruz.
	etag := strings.Trim(*output.ETag, "\"")
	return etag, nil
}

// DownloadObject, S3'teki bir nesneyi belirtilen yola indirmek için
// AWS SDK'sının 's3manager'ını kullanır. Bu, büyük dosyalar için daha verimlidir.
func (r *RealS3Client) DownloadObject(bucket, key, destinationPath string) error {
	file, err := os.Create(destinationPath)
	if err != nil {
		return fmt.Errorf("indirme hedefi oluşturulamadı (%s): %w", destinationPath, err)
	}
	defer file.Close()

	_, err = r.downloader.Download(context.TODO(), file, &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err != nil {
		os.Remove(destinationPath) // İndirme başarısız olursa, yarım kalan dosyayı sil.
		return fmt.Errorf("S3 DownloadObject (%s/%s) hatası: %w", bucket, key, err)
	}

	return nil
}
