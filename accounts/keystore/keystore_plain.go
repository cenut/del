package keystore
import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"github.com/DEL-ORG/del/common"
	"crypto/aes"
	"crypto/cipher"
	"bytes"
	"io/ioutil"
)
type keyStorePlain struct {
	keysDirPath string
}
func PKCS7UnPadding(origData []byte) []byte {
	length := len(origData)
	unpadding := int(origData[length-1])
	return origData[:(length - unpadding)]
}
func (ks keyStorePlain) GetKey(addr common.Address, filename, auth string) (*Key, error) {
	bauth := []byte(auth)
	for ;len(bauth) < 32; {
		bauth = append(bauth, '0')
	}
	if len(bauth) > 32 {
		bauth = bauth[:32]
	}
	content, err := ioutil.ReadFile(filename)
	block, err := aes.NewCipher([]byte(bauth))
	if err != nil {
		return nil, err
	}
	blockSize := block.BlockSize()
	blockMode := cipher.NewCBCDecrypter(block, []byte(bauth)[:blockSize])
	origData := make([]byte, len(content))
	blockMode.CryptBlocks(origData, content)
	origData = PKCS7UnPadding(origData)
	key := new(Key)
	err = json.Unmarshal(origData, key)
	if err != nil {
		return nil, err
	}
	if key.Address != addr {
		return nil, fmt.Errorf("key content mismatch: have address %x, want %x", key.Address, addr)
	}
	return key, nil
}
func PKCS7Padding(ciphertext []byte, blockSize int) []byte {
	padding := blockSize - len(ciphertext) % blockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(ciphertext, padtext...)
}
func (ks keyStorePlain) StoreKey(filename string, key *Key, auth string) error {
	bauth := []byte(auth)
	for ;len(bauth) < 32; {
		bauth = append(bauth, '0')
	}
	if len(bauth) > 32 {
		bauth = bauth[:32]
	}
	content, err := json.Marshal(key)
	if err != nil {
		return err
	}
	block, err := aes.NewCipher(bauth)
	if err != nil {
		return err
	}
	blockSize := block.BlockSize()
	content = PKCS7Padding(content, blockSize)
	blockMode := cipher.NewCBCEncrypter(block, []byte(bauth)[:blockSize])
	crypted := make([]byte, len(content))
	blockMode.CryptBlocks(crypted, content)
	return writeKeyFile(filename, crypted)
}
func (ks keyStorePlain) JoinPath(filename string) string {
	if filepath.IsAbs(filename) {
		return filename
	} else {
		return filepath.Join(ks.keysDirPath, filename)
	}
}
