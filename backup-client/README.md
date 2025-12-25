# Backup Client

Wails (Go + React) ile yazilmis masaustu yedekleme istemcisi. Time Machine tarzinda dosya versiyonlama ve geri yukleme ozellikleri sunar.

## Gereksinimler

- Go 1.21+
- Node.js 18+
- Wails CLI v2

### Wails Kurulumu

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

### Linux Ek Bagimliliklari

```bash
# Debian/Ubuntu
sudo apt install libgtk-3-dev libwebkit2gtk-4.0-dev

# Fedora
sudo dnf install gtk3-devel webkit2gtk3-devel
```

## Derleme

### Gelistirme Modu

```bash
cd backup-client
wails dev
```

### Uretim Derlemesi

```bash
wails build
```

Cikti: `build/bin/backup-client`

## Kullanim

### Ilk Kurulum

1. Uygulamayi calistirin
2. Sunucu adresini girin (ornek: `http://localhost:8080`)
3. Kayit olun veya giris yapin
4. Cihaz adi belirleyin

### Yedekleme

1. "Yedekleme" sekmesine gidin
2. Yedeklenecek dizinleri secin
3. "Yedekleme Baslat" butonuna tiklayin

### Geri Yukleme (Time Machine)

1. "Geri Yukleme" sekmesine gidin
2. Zaman cizelgesinden tarih secin
3. Dosya tarayicisinda gezinin
4. Dosya veya dizin secip "Geri Yukle" tiklayin

## Ozellikler

- **Incremental Yedekleme**: Sadece degisen dosyalar yedeklenir
- **AES Sifreleme**: Dosyalar sifrelenerek gonderilir
- **Yerel Katalog**: Hizli dosya tarama icin SQLite katalog
- **Katalog Kurtarma**: Sunucudan katalog yeniden olusturulabilir
- **Coklu Cihaz**: Birden fazla cihazi yonetebilme

## Yapilandirma

Ayarlar `~/.config/backup-client/` dizininde saklanir:

- `config.json`: Sunucu adresi, token
- `catalog.db`: Yerel dosya katalogu

## Dizin Yapisi

```
backup-client/
  app.go              # Wails uygulama mantigi
  main.go             # Giris noktasi
  internal/
    backup/
      service.go      # Yedekleme servisi
      catalog.go      # SQLite katalog
    crypto/
      aes.go          # AES sifreleme
  frontend/
    src/
      pages/
        Dashboard.tsx # Ana arayuz
        Login.tsx     # Giris ekrani
```

## Sorun Giderme

### Katalog bozuldu
"Katalogu Kurtar" butonuna tiklayin. Sunucudan yeniden olusturulur.

### Baglanti hatasi
Sunucu adresini ve internet baglantisini kontrol edin.

### Derleme hatasi
Wails bagimliliklanin kurulu oldugunu kontrol edin:
```bash
wails doctor
```

## Lisans

MIT License
