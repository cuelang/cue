package hmac

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"hash"
)

const (
	MD5        = "MD5"
	SHA1       = "SHA1"
	SHA224     = "SHA224"
	SHA256     = "SHA256"
	SHA384     = "SHA384"
	SHA512     = "SHA512"
	SHA512_224 = "SHA512_224"
	SHA512_256 = "SHA512_256"
)

func Sign(hashName string, data []byte, key []byte) ([]byte, error) {
	hash, err := hashFromName(hashName)
	if err != nil {
		return nil, err
	}
	mac := hmac.New(hash, key)
	mac.Write(data)
	return mac.Sum(nil), nil
}

func hashFromName(hash string) (func() hash.Hash, error) {
	switch hash {
	case MD5:
		return md5.New, nil
	case SHA1:
		return sha1.New, nil
	case SHA224:
		return sha256.New224, nil
	case SHA256:
		return sha256.New, nil
	case SHA384:
		return sha512.New384, nil
	case SHA512:
		return sha512.New, nil
	case SHA512_224:
		return sha512.New512_224, nil
	case SHA512_256:
		return sha512.New512_256, nil
	}
	return nil, fmt.Errorf("unsupported hash function")
}
