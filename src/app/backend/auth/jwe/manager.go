package jwe

import (
	"encoding/json"
	"errors"
	authApi "github.com/wzt3309/k8sconsole/src/app/backend/auth/api"
	kcErrors "github.com/wzt3309/k8sconsole/src/app/backend/errors"
	"gopkg.in/square/go-jose.v2"
	"k8s.io/client-go/tools/clientcmd/api"
	"time"
)

// Implements TokenManager interface.
type jweTokenManager struct {
	keyHolder KeyHolder
	tokenTTL  time.Duration
}

// AdditionalAuthData contains information required to validate token.
type AdditionalAuthData map[Claim]string

// Claim represent token claims used in AAD header.
type Claim string

const (
	timeFormat = time.RFC3339
	// IAT claim is part of token AAD header. It represents token "born" time.
	BOR Claim = "iat"
	// EXP claim is part of token AAD header. It represents token expiration time.
	EXP Claim = "exp"
)

// Generate adn encrypt JWE token based on provided AuthInfo. AuthInfo will be embedded in a token payload and
// encrypted with autogenerated signing key.
func (self *jweTokenManager) Generate(authInfo api.AuthInfo) (string, error) {
	marshalledAuthInfo, err := json.Marshal(authInfo)
	if err != nil {
		return "", err
	}

	jweObject, err := self.getEncrypter().EncryptWithAuthData(marshalledAuthInfo, self.generateADD())
	if err != nil {
		return "", err
	}

	return jweObject.FullSerialize(), nil
}

// Decrypt provides token and returns AuthInfo saved in a token payload.
func (self *jweTokenManager) Decrypt(jweToken string) (*api.AuthInfo, error) {
	jweTokenObject, err := self.validate(jweToken)
	if err != nil {
		return nil, err
	}

	decryted, err := jweTokenObject.Decrypt(self.keyHolder.Key())
	if err == jose.ErrCryptoFailure {
		// Force key refresh and try to decrypt again
		// TODO(wzt3309) Now keyHolder refresh method is not ready for use
		self.keyHolder.Refresh()
		decryted, err = jweTokenObject.Decrypt(self.keyHolder.Key())
	}

	if err != nil {
		return nil, err
	}

	authInfo := new(api.AuthInfo)
	err = json.Unmarshal(decryted, authInfo)
	return authInfo, err
}

// Refresh implements TokenManager interface
func (self *jweTokenManager) Refresh(jweToken string) (string, error) {
	if len(jweToken) == 0 {
		return "", errors.New("Can not refresh token. No token provided.")
	}

	jweTokenObject, err := self.validate(jweToken)
	if err != nil {
		return "", err
	}

	decrypted, err := jweTokenObject.Decrypt(self.keyHolder.Key())
	if err != nil {
		return "", err
	}

	authInfo := new(api.AuthInfo)
	err = json.Unmarshal(decrypted, authInfo)
	if err != nil {
		return "", errors.New("Token refresh error. Could not unmarshal token payload.")
	}

	return self.Generate(*authInfo)
}

// SetTokenTTL implements TokenManager interface
func (self *jweTokenManager) SetTokenTTL(ttl time.Duration) {
	if ttl < 0 {
		ttl = 0
	}

	self.tokenTTL = ttl * time.Second
}

func (self *jweTokenManager) getEncrypter() jose.Encrypter {
	return self.keyHolder.Encrypter()
}

// Parses and validates provided token to check if it is expired
func (self *jweTokenManager) validate(jweToken string) (*jose.JSONWebEncryption, error) {
	jwe, err := jose.ParseEncrypted(jweToken)
	if err != nil {
		return nil, err
	}

	if self.tokenTTL > 0 {
		aad := AdditionalAuthData{}
		err = json.Unmarshal(jwe.GetAuthData(), &aad)
		if err != nil {
			return nil, errors.New("Token validation error. Could not unmarshal AAD.")
		}

		if self.isExpired(aad[BOR], aad[EXP]) {
			return nil, errors.New(kcErrors.MSG_TOKEN_EXPIRED_ERROR)
		}
	}

	return jwe, nil
}

// Retures true if token has expired.
// If the time string can't be parsed into time, the token will be marked as expired.
func (self *jweTokenManager) isExpired(borStr, expStr string) bool {
	bor, err := time.Parse(timeFormat, borStr)
	if err != nil {
		return true
	}

	exp, err := time.Parse(timeFormat, expStr)
	if err != nil {
		return true
	}

	age := time.Now().Sub(bor.Local())
	return bor.Add(age).After(exp)
}

func (self *jweTokenManager) generateADD() []byte {
	now := time.Now()
	aad := AdditionalAuthData{
		BOR: now.Format(timeFormat),
	}

	if self.tokenTTL > 0 {
		aad[EXP] = now.Add(self.tokenTTL).Format(timeFormat)
	}

	rawAAD, _ := json.Marshal(aad)
	return rawAAD
}

// Creates and returns default JWE Token manager interface
func NewJWETokenManager(holder KeyHolder) authApi.TokenManager {
	manager := &jweTokenManager{keyHolder: holder, tokenTTL: authApi.DefaultTokenTTL * time.Second}
	return manager
}
