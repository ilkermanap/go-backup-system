package crypto

import (
	"bytes"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"golang.org/x/crypto/pbkdf2"
)

const (
	// PBKDF2 parameters for secure key derivation
	pbkdf2Iterations = 100000
	saltSize         = 32
	keySize          = 32 // AES-256
)

// DeriveKeyWithSalt derives a 32-byte AES-256 key from passphrase using PBKDF2
func DeriveKeyWithSalt(passphrase string, salt []byte) []byte {
	return pbkdf2.Key([]byte(passphrase), salt, pbkdf2Iterations, keySize, sha256.New)
}

// DeriveKey derives a 32-byte key from a password using SHA256 (simple version)
func DeriveKey(password string) []byte {
	hash := sha256.Sum256([]byte(password))
	return hash[:]
}

// GenerateSalt generates a random salt for key derivation
func GenerateSalt() ([]byte, error) {
	salt := make([]byte, saltSize)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, err
	}
	return salt, nil
}

// HashPath returns SHA-224 hash of a file path (for filename obfuscation)
// This is used to hide original filenames from the server
func HashPath(path string) string {
	h := sha256.Sum224([]byte(path))
	return hex.EncodeToString(h[:])
}

// HashFileContent returns SHA-256 hash of file contents
func HashFileContent(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// HashData returns SHA-256 hash of byte data
func HashData(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// Encrypt encrypts data with AES-256-GCM
func Encrypt(data []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	return gcm.Seal(nonce, nonce, data, nil), nil
}

// Decrypt decrypts data with AES-256-GCM
func Decrypt(data []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

// CompressAndEncrypt compresses (gzip) then encrypts data
func CompressAndEncrypt(data []byte, key []byte) ([]byte, error) {
	// Compress with gzip
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(data); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}

	// Encrypt
	return Encrypt(buf.Bytes(), key)
}

// DecryptAndDecompress decrypts then decompresses data
func DecryptAndDecompress(data []byte, key []byte) ([]byte, error) {
	// Decrypt
	compressed, err := Decrypt(data, key)
	if err != nil {
		return nil, err
	}

	// Decompress
	gz, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, err
	}
	defer gz.Close()

	return io.ReadAll(gz)
}

// EncryptFile reads, compresses, encrypts and writes to destination
// Returns the encrypted file size
func EncryptFile(src, dst string, key []byte) (int64, error) {
	data, err := os.ReadFile(src)
	if err != nil {
		return 0, err
	}

	encrypted, err := CompressAndEncrypt(data, key)
	if err != nil {
		return 0, err
	}

	if err := os.WriteFile(dst, encrypted, 0600); err != nil {
		return 0, err
	}

	return int64(len(encrypted)), nil
}

// DecryptFile decrypts a file and writes to destination
func DecryptFile(src, dst string, key []byte) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	decrypted, err := DecryptAndDecompress(data, key)
	if err != nil {
		return err
	}

	return os.WriteFile(dst, decrypted, 0644)
}

// EncryptToHashedFile encrypts a file and saves with hashed filename
// Returns: hashed filename (without .enc), encrypted size, error
func EncryptToHashedFile(srcPath, destDir string, key []byte) (string, int64, error) {
	hashedName := HashPath(srcPath)
	destPath := fmt.Sprintf("%s/%s.enc", destDir, hashedName)

	size, err := EncryptFile(srcPath, destPath, key)
	if err != nil {
		return "", 0, err
	}

	return hashedName, size, nil
}
