# EdgeSync AI Agent v1.0 - Kullanım Kılavuzu

EdgeSync AI Agent, AWS S3 bucket'ınızdaki (kova) makine öğrenimi modellerinizi (veya herhangi bir dosyayı) izleyen ve yeni bir sürüm algılandığında bunları otomatik olarak "edge" (kenar) sunucularınıza dağıtan (deploy eden) hafif, platform bağımsız bir Go ajanıdır.

## Özellikler (v1.0)

* **Platform Bağımsız:** Tek bir binary, Windows, Linux ve macOS üzerinde çalışır.
* **Güvenli:** `config.json` içine asla gizli anahtar (secret key) yazmaz. AWS IAM Rolleri veya Ortam Değişkenleri (`$env:`) ile çalışır.
* **Hafif:** Go ile yazılmıştır, minimum CPU ve RAM kullanır.
* **Esnek:** `deploy` script'i (`.bat` veya `.sh`) sayesinde Docker, Python, systemd veya herhangi bir özel servisle entegre olabilir.
* **Dayanıklı:** `deploy` script'iniz hata verirse, ajan dağıtımı otomatik olarak geri alır (rollback) (eğer önceki bir sürüm varsa).
* **İzlenebilir:** `http://localhost:8080` üzerinden basit bir durum paneli sunar.

---

## Hızlı Kurulum (5 Adım)

### Adım 1: AWS Hazırlığı (S3 Bucket ve IAM Kullanıcısı)

Ajanın "en az yetki" (least privilege) ilkesiyle çalışması için, S3'e özel bir "robot" kullanıcı (IAM User) oluşturmanız gerekir.

**1. S3 Bucket'ı Oluşturun:**
   * AWS S3 konsoluna gidin ve modellerinizi saklamak için bir bucket oluşturun (örn: `my-model-bucket`).

**2. IAM Politikası (Kural) Oluşturun:**
   * AWS IAM > Policies > "Create policy" > "JSON" sekmesine gidin.
   * Aşağıdaki JSON'u yapıştırın (ve `YOUR_BUCKET_NAME_HERE` kısmını kendi bucket adınızla değiştirin):
     ```json
     {
       "Version": "2012-10-17",
       "Statement": [
         {
           "Effect": "Allow",
           "Action": "s3:GetObject",
           "Resource": "arn:aws:s3:::YOUR_BUCKET_NAME_HERE/*"
         }
       ]
     }
     ```
   * Politikayı `EdgeSync-S3-ReadOnly-Policy` gibi bir isimle kaydedin.

**3. IAM Kullanıcısı (Robot) Oluşturun:**
   * AWS IAM > Users > "Add users" deyin.
   * Kullanıcı adı olarak `edgesync-agent-user` verin.
   * "Attach policies directly" (Politikaları doğrudan ekle) seçin ve az önce oluşturduğunuz `EdgeSync-S3-ReadOnly-Policy`'yi seçip kullanıcıyı oluşturun.

**4. Erişim Anahtarı (Access Key) Alın:**
   * `edgesync-agent-user` kullanıcısının detaylarına gidin.
   * "Security credentials" > "Access keys" > "Create access key" deyin.
   * "Command Line Interface (CLI)" seçeneğini seçin.
   * **ÖNEMLİ:** "I understand the recommendation..." (Tavsiyeyi anlıyorum...) onay kutusunu işaretleyip anahtarınızı oluşturun.
   * Size verilen **Access key ID** ve **Secret access key**'i bir yere not edin.

### Adım 2: Ajanı Sunucuya Kurun

1.  Bu projenin "Releases" (Sürümler) sayfasından işletim sisteminize uygun paketi (`.zip`) indirin.
2.  İçeriği sunucunuzda kalıcı bir yere (örn: `C:\EdgeSync` veya `/opt/edgesync`) çıkartın.
3.  Paketin içindekiler:
    * `edgesync-agent.exe` (Windows için) VEYA `edgesync-agent-linux` (Linux için)
    * `config.json.example`
    * `deploy.bat.example` (Windows için)
    * `deploy.sh.example` (Linux için)
    * `README.md` (Bu dosya)

### Adım 3: `config.json`'u Yapılandırın

1.  `config.json.example` dosyasının adını **`config.json`** olarak değiştirin.
2.  Dosyayı açıp kendi ayarlarınızla doldurun:
    ```json
    {
      "s3_bucket": "my-model-bucket",
      "s3_key": "prod/latest_model.bin",
      "deploy_script_path": "./deploy.sh"
    }
    ```
   * **deploy_script_path:** Windows için `.\\deploy.bat`, Linux/macOS için `./deploy.sh` kullanın.

### Adım 4: `deploy` Script'ini Hazırlayın

1.  İşletim sisteminize uygun `.example` script'inin adını (`deploy.bat` veya `deploy.sh`) değiştirin.
2.  İçini kendi dağıtım mantığınıza göre düzenleyin (örn: `docker-compose restart` veya `systemctl restart my-service`).
3.  **Linux/macOS için:** Script'i çalıştırılabilir yapın: `chmod +x ./deploy.sh`

### Adım 5: Ajanı Çalıştırın

Ajanın çalışabilmesi için Adım 1'de aldığınız AWS anahtarlarını "Ortam Değişkeni" (Environment Variable) olarak bilmesi gerekir.

**Windows (PowerShell):**
```powershell
# Anahtarları o terminal oturumu için ayarla
$env:AWS_ACCESS_KEY_ID = "SIZIN_ACCESS_KEY_ID"
$env:AWS_SECRET_ACCESS_KEY = "SIZIN_SECRET_ACCESS_KEY"
$env:AWS_DEFAULT_REGION = "us-east-1"

# ÖNEMLİ: Sembolik bağ (symlink) oluşturabilmek için 
# PowerShell'i "Yönetici Olarak" (Run as Administrator) çalıştırmanız gerekir.
.\\edgesync-agent.exe

#LINUX/macOS
# Anahtarları o terminal oturumu için ayarla
export AWS_ACCESS_KEY_ID="SIZIN_ACCESS_KEY_ID"
export AWS_SECRET_ACCESS_KEY="SIZIN_SECRET_ACCESS_KEY"
export AWS_DEFAULT_REGION="us-east-1"

# Ajanı çalıştır (Linux'ta 'sudo' gerekmez)
./edgesync-agent-linux

#Ajanınız şimdi çalışıyor!
#Durumu izlemek için tarayıcınızdan http://localhost:8080 adresine gidin.