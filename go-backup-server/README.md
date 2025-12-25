# Go Backup Server

Go ve Gin framework ile yazilmis yedekleme sunucusu.

## Gereksinimler

- Go 1.21+
- SQLite3

## Kurulum

### Kaynak Koddan Derleme

```bash
cd go-backup-server
go mod download
go build -o backup-server ./cmd/server
```

### Yapilandirma

`config.yaml` dosyasi olusturun:

```yaml
server:
  port: 8080
  host: "0.0.0.0"

database:
  path: "./backup.db"

storage:
  path: "./storage"

jwt:
  secret: "your-secret-key-here"
  expiry: 24h
```

## Calistirma

```bash
./backup-server
```

Veya ortam degiskenleri ile:

```bash
PORT=8080 STORAGE_PATH=/data/backups ./backup-server
```

## API Endpoints

### Kimlik Dogrulama

| Endpoint | Method | Aciklama |
|----------|--------|----------|
| `/api/auth/register` | POST | Yeni kullanici kaydi |
| `/api/auth/login` | POST | Giris ve JWT token alma |

### Yedekleme

| Endpoint | Method | Aciklama |
|----------|--------|----------|
| `/api/backup/upload` | POST | Dosya yukleme |
| `/api/backup/restore` | POST | Dosya geri yukleme |
| `/api/backup/catalog` | GET | Katalog bilgisi |

### Cihaz Yonetimi

| Endpoint | Method | Aciklama |
|----------|--------|----------|
| `/api/devices` | GET | Cihaz listesi |
| `/api/devices` | POST | Yeni cihaz ekleme |
| `/api/devices/:id` | DELETE | Cihaz silme |

## Dizin Yapisi

Yedekler asagidaki yapida saklanir:

```
storage/
  {user_hash}/
    {device_id}/
      {timestamp}/           # Format: 20060102-150405
        file1.tar
        file2.tar
```

## Veritabani Semasi

### users
- id, email, password_hash, created_at, quota_bytes

### devices
- id, user_id, name, created_at

### files (katalog)
- id, device_id, orig_path, hashed_name, size, hash, timestamp

## Guvenlik

- JWT tabanli kimlik dogrulama
- Sifrelenmis dosya transferi (AES)
- Kullanici bazli izolasyon (user_hash ile)

## Lisans

MIT License
