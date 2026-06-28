# Event-Driven Notification System

## Proje Özeti

Bu proje, SMS, Email ve Push kanalında bildirim gönderebilen ölçeklenebilir bir bildirim sistemi sunar. Sistem, bildirimi alır, veritabanına kaydeder, öncelik sırasına göre kuyrukta işler ve webhook üzerinden dış sağlayıcıya gönderir.

## Bileşenler

### 1. API Sunucusu
- `source/cmd/notification-server/main.go`
  - Sunucu başlatma ve graceful shutdown
  - Yapılandırma yükleme
  - router oluşturma

### 2. Yapılandırma
- `source/config/config.go`
  - `SERVER_ADDRESS`, `DATABASE_URL`, `WEBHOOK_URL` ortam değişkenleri
  - Zorunlu webhook URL kontrolü

### 3. Veri Modeli
- `source/internal/model/notification.go`
  - Bildirim durumu ve öncelik sabitleri
  - Bildirim yapısı (`Notification`)
  - Kanal ve içerik doğrulamaları

### 4. Depolama Katmanı
- `source/internal/storage/postgres.go`
  - PostgreSQL bağlantısı
  - Schema migration
  - Transaction ile batch bildirim kaydetme, güncelleme, sorgulama ve iptal etme
  - Arama filtreleme ve sayfalama

### 5. Kuyruk Yönetimi
- `source/internal/queue/queue.go`
  - Öncelikli kuyruk implementasyonu
  - Kanal bazlı işleyici döngüleri
  - Bildirim işleme ve gönderme mantığı
  - Kuyruk derinliği ölçümü

### 6. Sağlayıcı Entegrasyonu
- `source/internal/provider/provider.go`
  - Webhook.site gibi dış sağlayıcıya POST isteği gönderme
  - 200 veya 202 yanıtlarını kabul etme
  - JSON yanıtı olmadığında UUID üretme

### 7. Metrik ve Gözlemlenebilirlik
- `source/internal/metrics/metrics.go`
  - Sıra derinliği
  - Başarı ve başarısızlık sayıları
  - Son güncelleme zamanı
  - JSON olarak metric çıktısı üretme

### 8. API Yönlendirmesi
- `source/internal/api/router.go`
  - Bildirim oluşturma: `POST /notifications`
  - Durum sorgulama: `GET /notifications/{id}`
  - Listeleme filtrasyon: `GET /notifications`
  - İptal etme: `DELETE /notifications/{id}`
  - Sağlık kontrolü: `GET /health`
  - Metric: `GET /metrics`

## Kullanım

1. `source` dizinine gidin.
2. Ortam değişkenlerini ayarlayın:
```bash
export WEBHOOK_URL="https://webhook.site/fa8d1250-1966-4b1a-91f5-4c2847138075"
export SERVER_ADDRESS=":8080"
export DATABASE_URL="postgres://notification:notification@localhost:5432/notifications?sslmode=disable"
```
3. Veritabanı klasörünü oluşturun:
```bash
mkdir -p source/data
```
4. Sunucuyu çalıştırın:
```bash
go build -o notification-server ./cmd/notification-server
./notification-server
```

## Özellikler

- Toplu bildirim oluşturma (maksimum 1000)
- Filtreleme ve sayfalama
- Kanala göre öncelik sıralaması
- PostgreSQL primary key ile korunan idempotentlik desteği
- Planlı bildirim gönderimi (`scheduled_at`)
- Şablon sistemi ve değişken yerleştirme
- Gerçek zamanlı metrikler
- Sağlık kontrolü
- Dış webhook entegrasyonu

## Geliştirme Notları

- `docker-compose.yml` ve `Dockerfile` mevcut
- `README.md` ve `IMPLEMENTATION.md` proje dokümantasyonu sağlar
- `test.sh` temel uçtan uca testi kolaylaştırır
- Veri saklama PostgreSQL üzerinden yapılır; yüksek trafik ve eşzamanlı yazma için SQLite'a göre daha uygundur
- Sistemin tek instans olduğu; dağıtık kuyruğa ihtiyaç varsa Redis veya RabbitMQ eklenebilir
- Loglar JSON structured formatta üretilir ve correlation ID alanı taşır

## Dikkat Edilmesi Gerekenler

- webhook.site varsayılan yanıtları 200 dönebilir, bu nedenle provider tarafında hem 200 hem 202 kabul ediliyor.
- `idempotency_key` alanı boşsa `NULL` olarak kaydediliyor; dolu olduğunda `idempotency_keys` tablosu duplicate batch isteklerini atomik olarak engelliyor.
- Posta gönderimleri dış sağlayıcıya gerçekten iletilmiyor; webhook.site ile simülasyon sağlanıyor.

## Geliştirme Adımları

- Dead-letter queue tablosu eklenebilir
- Kuyruk kalıcılığı için Redis/RabbitMQ kullanılabilir
- API kimlik doğrulaması eklenebilir
- Prometheus/Grafana entegrasyonu ile monitoring genişletilebilir
- WebSocket durum güncellemeleri ve distributed tracing eklenebilir
