package crypto

import (
	"bytes"
	"os"
	"testing"
)

// unsetenv removes an env var and registers cleanup to restore it.
func unsetenv(t *testing.T, key string) {
	t.Helper()
	orig, exists := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("failed to unset %s: %v", key, err)
	}
	t.Cleanup(func() {
		if exists {
			os.Setenv(key, orig)
		}
	})
}

func TestIsEncrypted(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{
			name: "valid magic bytes",
			data: append(MagicBytes, 0x01, 0x02, 0x03),
			want: true,
		},
		{
			name: "exact magic bytes",
			data: MagicBytes,
			want: true,
		},
		{
			name: "plain YAML",
			data: []byte("version: 1\nvariables:\n"),
			want: false,
		},
		{
			name: "empty data",
			data: []byte{},
			want: false,
		},
		{
			name: "nil data",
			data: nil,
			want: false,
		},
		{
			name: "too short",
			data: []byte("VAR"),
			want: false,
		},
		{
			name: "similar but wrong magic",
			data: []byte("VARNISH!"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsEncrypted(tt.data)
			if got != tt.want {
				t.Errorf("IsEncrypted() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetPassword(t *testing.T) {
	tests := []struct {
		name    string
		envVal  string
		setEnv  bool
		want    string
		wantErr bool
	}{
		{
			name:    "password set",
			envVal:  "mysecretpassword",
			setEnv:  true,
			want:    "mysecretpassword",
			wantErr: false,
		},
		{
			name:    "password empty",
			envVal:  "",
			setEnv:  true,
			want:    "",
			wantErr: true,
		},
		{
			name:    "password not set",
			envVal:  "",
			setEnv:  false,
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setEnv {
				t.Setenv(PasswordEnvVar, tt.envVal)
			} else {
				unsetenv(t, PasswordEnvVar)
			}

			got, err := GetPassword()
			if (err != nil) != tt.wantErr {
				t.Errorf("GetPassword() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetPassword() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDeriveKey(t *testing.T) {
	password := "testpassword"
	salt := []byte("1234567890123456") // 16 bytes

	key := DeriveKey(password, salt)

	// Key should be 32 bytes (256 bits)
	if len(key) != 32 {
		t.Errorf("DeriveKey() returned %d bytes, want 32", len(key))
	}

	// Same inputs should produce same output
	key2 := DeriveKey(password, salt)
	if !bytes.Equal(key, key2) {
		t.Error("DeriveKey() not deterministic")
	}

	// Different password should produce different key
	key3 := DeriveKey("different", salt)
	if bytes.Equal(key, key3) {
		t.Error("DeriveKey() same result for different passwords")
	}

	// Different salt should produce different key
	key4 := DeriveKey(password, []byte("6543210987654321"))
	if bytes.Equal(key, key4) {
		t.Error("DeriveKey() same result for different salts")
	}
}

func TestEncryptDecrypt(t *testing.T) {
	tests := []struct {
		name      string
		plaintext string
		password  string
	}{
		{
			name:      "simple text",
			plaintext: "hello world",
			password:  "testpassword",
		},
		{
			name:      "empty plaintext",
			plaintext: "",
			password:  "testpassword",
		},
		{
			name:      "yaml content",
			plaintext: "version: 1\nvariables:\n  database.host: localhost\n  database.port: \"5432\"\n",
			password:  "mysecret",
		},
		{
			name:      "unicode content",
			plaintext: "Hello, \u4e16\u754c! \u00e9\u00e8\u00e0",
			password:  "password123",
		},
		{
			name:      "long content",
			plaintext: string(make([]byte, 10000)),
			password:  "longpassword",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encrypted, err := Encrypt([]byte(tt.plaintext), tt.password)
			if err != nil {
				t.Fatalf("Encrypt() error = %v", err)
			}

			// Encrypted data should start with magic bytes
			if !IsEncrypted(encrypted) {
				t.Error("Encrypt() output doesn't start with magic bytes")
			}

			// Encrypted data should be larger than plaintext
			minExpected := len(MagicBytes) + 1 + saltSize + nonceSize + len(tt.plaintext) + 16
			if len(encrypted) < minExpected {
				t.Errorf("Encrypt() output too small: got %d, want >= %d", len(encrypted), minExpected)
			}

			// Decrypt should return original
			decrypted, err := Decrypt(encrypted, tt.password)
			if err != nil {
				t.Fatalf("Decrypt() error = %v", err)
			}

			if string(decrypted) != tt.plaintext {
				t.Errorf("Decrypt() = %q, want %q", string(decrypted), tt.plaintext)
			}
		})
	}
}

func TestEncryptProducesDifferentOutput(t *testing.T) {
	plaintext := []byte("same content")
	password := "samepassword"

	// Encrypt same content twice
	enc1, err := Encrypt(plaintext, password)
	if err != nil {
		t.Fatalf("first Encrypt() error = %v", err)
	}

	enc2, err := Encrypt(plaintext, password)
	if err != nil {
		t.Fatalf("second Encrypt() error = %v", err)
	}

	// Should produce different ciphertext (due to random salt and nonce)
	if bytes.Equal(enc1, enc2) {
		t.Error("Encrypt() produced identical output for same input (should use random salt/nonce)")
	}

	// Both should decrypt to same plaintext
	dec1, _ := Decrypt(enc1, password)
	dec2, _ := Decrypt(enc2, password)
	if !bytes.Equal(dec1, dec2) {
		t.Error("different encrypted versions decrypt to different plaintext")
	}
}

func TestDecryptWrongPassword(t *testing.T) {
	plaintext := []byte("secret data")
	password := "correctpassword"
	wrongPassword := "wrongpassword"

	encrypted, err := Encrypt(plaintext, password)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	_, err = Decrypt(encrypted, wrongPassword)
	if err == nil {
		t.Error("Decrypt() with wrong password should fail")
	}
}

func TestDecryptEmptyPassword(t *testing.T) {
	plaintext := []byte("secret data")
	password := "testpassword"

	encrypted, err := Encrypt(plaintext, password)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	_, err = Decrypt(encrypted, "")
	if err == nil {
		t.Error("Decrypt() with empty password should fail")
	}
	if err != ErrPasswordRequired {
		t.Errorf("Decrypt() error = %v, want ErrPasswordRequired", err)
	}
}

func TestDecryptCorruptedData(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{
			name: "too short",
			data: MagicBytes,
		},
		{
			name: "missing magic",
			data: []byte("not encrypted data here"),
		},
		{
			name: "wrong version",
			data: func() []byte {
				d := make([]byte, len(MagicBytes)+1+saltSize+nonceSize+32)
				copy(d, MagicBytes)
				d[len(MagicBytes)] = 99 // Invalid version
				return d
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Decrypt(tt.data, "anypassword")
			if err == nil {
				t.Error("Decrypt() should fail on corrupted data")
			}
		})
	}
}

func TestDecryptTamperedCiphertext(t *testing.T) {
	plaintext := []byte("important data")
	password := "testpassword"

	encrypted, err := Encrypt(plaintext, password)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	// Tamper with the ciphertext (last byte)
	encrypted[len(encrypted)-1] ^= 0xFF

	_, err = Decrypt(encrypted, password)
	if err == nil {
		t.Error("Decrypt() should fail on tampered ciphertext (GCM auth should catch this)")
	}
}
