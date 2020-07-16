package util

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/csv"
	"encoding/pem"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"net/http"
	"net/mail"
	"os"
	"regexp"
	"time"

	log "github.com/gophish/gophish/logger"
	"github.com/gophish/gophish/models"
	"github.com/jordan-wright/email"
	"golang.org/x/crypto/bcrypt"
	"github.com/jinzhu/gorm"
)

var (
	firstNameRegex = regexp.MustCompile(`(?i)first[\s_-]*name`)
	lastNameRegex  = regexp.MustCompile(`(?i)last[\s_-]*name`)
	emailRegex     = regexp.MustCompile(`(?i)email`)
	positionRegex  = regexp.MustCompile(`(?i)position`)
	
	groupRegex 	   = regexp.MustCompile(`(?i)group`)
	fieldRegex 	   = regexp.MustCompile(`(?i)field`)
	
	conditionRegex = regexp.MustCompile(`(?i)condition`)
	valueRegex     = regexp.MustCompile(`(?i)value`)
)

// ParseMail takes in an HTTP Request and returns an Email object
// TODO: This function will likely be changed to take in a []byte
func ParseMail(r *http.Request) (email.Email, error) {
	e := email.Email{}
	m, err := mail.ReadMessage(r.Body)
	if err != nil {
		return e, err
	}
	body, err := ioutil.ReadAll(m.Body)
	e.HTML = body
	return e, err
}

// ParseCSV contains the logic to parse the user provided csv file containing Target entries
func ParseCSV(r *http.Request) ([]models.Target, error) {
	mr, err := r.MultipartReader()
	ts := []models.Target{}
	if err != nil {
		return ts, err
	}
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		// Skip the "submit" part
		if part.FileName() == "" {
			continue
		}
		defer part.Close()
		reader := csv.NewReader(part)
		reader.TrimLeadingSpace = true
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		fi := -1
		li := -1
		ei := -1
		pi := -1
		fn := ""
		ln := ""
		ea := ""
		ps := ""
		for i, v := range record {
			switch {
			case firstNameRegex.MatchString(v):
				fi = i
			case lastNameRegex.MatchString(v):
				li = i
			case emailRegex.MatchString(v):
				ei = i
			case positionRegex.MatchString(v):
				pi = i
			}
		}
		if fi == -1 && li == -1 && ei == -1 && pi == -1 {
			continue
		}
		for {
			record, err := reader.Read()
			if err == io.EOF {
				break
			}
			if fi != -1 && len(record) > fi {
				fn = record[fi]
			}
			if li != -1 && len(record) > li {
				ln = record[li]
			}
			if ei != -1 && len(record) > ei {
				csvEmail, err := mail.ParseAddress(record[ei])
				if err != nil {
					continue
				}
				ea = csvEmail.Address
			}
			if pi != -1 && len(record) > pi {
				ps = record[pi]
			}
			t := models.Target{
				BaseRecipient: models.BaseRecipient{
					FirstName: fn,
					LastName:  ln,
					Email:     ea,
					Position:  ps,
				},
			}
			ts = append(ts, t)
		}
	}
	return ts, nil
}

// ParseCSVValues contains the logic to parse the user provided csv file containing FieldValue entries
func ParseCSVValues(r *http.Request) ([]models.FieldValue, error) {
	mr, err := r.MultipartReader()
	vs := []models.FieldValue{}
	if err != nil {
		return vs, err
	}
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		// Skip the "submit" part
		if part.FileName() == "" {
			continue
		}
		defer part.Close()
		reader := csv.NewReader(part)
		reader.TrimLeadingSpace = true
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		ei := -1
		vi := -1
		ea := ""
		vn := ""
		for i, v := range record {
			switch {
			case emailRegex.MatchString(v):
				ei = i
			case valueRegex.MatchString(v):
				vi = i
			}
		}
		if ei == -1 || vi == -1 {
			continue
		}
		for {
			record, err := reader.Read()
			if err == io.EOF {
				break
			}
			if ei != -1 && len(record) > ei {
				csvEmail, err := mail.ParseAddress(record[ei])
				if err != nil {
					continue
				}
				ea = csvEmail.Address
			}
			if vi != -1 && len(record) > vi {
				vn = record[vi]
			}
			v := models.FieldValue{
				Email: ea,
				Value: vn,
			}
			vs = append(vs, v)
		}
	}
	return vs, nil
}

// ParseCSVConditions contains the logic to parse the user provided csv file containing Condition entries
func ParseCSVConditions(r *http.Request) ([]models.Condition, error) {
	mr, err := r.MultipartReader()
	cs := []models.Condition{}
	if err != nil {
		return cs, err
	}
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		// Skip the "submit" part
		if part.FileName() == "" {
			continue
		}
		defer part.Close()
		reader := csv.NewReader(part)
		reader.TrimLeadingSpace = true
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		ci := -1
		vi := -1
		cn := ""
		vn := ""
		for i, c := range record {
			switch {
			case conditionRegex.MatchString(c):
				ci = i
			case valueRegex.MatchString(c):
				vi = i
			}
		}
		if ci == -1 || vi == -1 {
			continue
		}
		for {
			record, err := reader.Read()
			if err == io.EOF {
				break
			}
			if ci != -1 && len(record) > ci {
				cn = record[ci]
			}
			if vi != -1 && len(record) > vi {
				vn = record[vi]
			}
			c := models.Condition{
				Condition: cn,
				Value: vn,
			}
			cs = append(cs, c)
		}
	}
	return cs, nil
}

// ParseCSVInfo contains the logic to parse the user provided csv file containing information
func ParseCSVInfo(r *http.Request) ([]models.FieldValue, error) {
	mr, err := r.MultipartReader()
	vs := []models.FieldValue{}
	if err != nil {
		return vs, err
	}
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		// Skip the "submit" part
		if part.FileName() == "" {
			continue
		}
		defer part.Close()
		reader := csv.NewReader(part)
		reader.TrimLeadingSpace = true
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		gi := -1
		ei := -1
		fi := -1
		vi := -1
		gn := ""
		en := ""
		fn := ""
		vn := ""
		for i, c := range record {
			switch {
			case groupRegex.MatchString(c):
				gi = i
			case emailRegex.MatchString(c):
				ei = i
			case fieldRegex.MatchString(c):
				fi = i
			case valueRegex.MatchString(c):
				vi = i
			}
		}
		if gi == -1 || ei == -1 || fi == -1 || vi == -1 {
			continue
		}
		for {
			record, err := reader.Read()
			if err == io.EOF {
				break
			}
			if gi != -1 && len(record) > gi {
				gn = record[gi]
			}
			if ei != -1 && len(record) > ei {
				csvEmail, err := mail.ParseAddress(record[ei])
				if err != nil {
					continue
				}
				en = csvEmail.Address
			}
			if fi != -1 && len(record) > fi {
				fn = record[fi]
			}
			if vi != -1 && len(record) > vi {
				vn = record[vi]
			}
			
			user_id := int64(1)
			
			target, err := models.GetTargetByEmail(en)
			if err != nil && err != gorm.ErrRecordNotFound {
				continue
			}
			
			// If the target doesn't exist
			if err == gorm.ErrRecordNotFound {
			
				group, err := models.GetGroupByName(gn, user_id)
				if err != nil && err != gorm.ErrRecordNotFound {
					continue
				}
				
				// If the group doesn't exist
				if err == gorm.ErrRecordNotFound {
				
					// Create the group and the target
					group.UserId = user_id
					group.Name = gn
					group.ModifiedDate = time.Now().UTC()
					
					targets := []models.Target{}
					bs := models.BaseRecipient{Email: en}
					targets = append(targets, models.Target{BaseRecipient: bs})
					
					group.Targets = targets
					
					models.PostGroup(&group)
					group, err = models.GetGroupByName(gn, user_id)
					if err != nil {
						continue
					}
				
				// Else if the group exists
				} else {
				
					// Create the target
					bs := models.BaseRecipient{Email: en}
					group.Targets = append(group.Targets, models.Target{BaseRecipient: bs})
					models.PutGroup(&group)
				}
				
				target, err = models.GetTargetByEmail(en)
				if err != nil {
					continue
				}
			}
			
			// If the target exists	
			if target.Id != 0 {
				
				field, err := models.GetFieldByName(fn, user_id)
				if err != nil && err != gorm.ErrRecordNotFound {
					continue
				}
				
				// If the field doesn't exist
				if err == gorm.ErrRecordNotFound {
				
					// Create the field and the value
					field.UserId = user_id
					field.Name = fn
					field.ModifiedDate = time.Now().UTC()
					
					values := []models.FieldValue{}
					values = append(values, models.FieldValue{Email: en, Value: vn})
					
					field.FieldValues = values
					
					models.PostField(&field)
					field, err = models.GetFieldByName(fn, user_id)
					if err != nil {
						continue
					}
					
				// Else if the field exists	
				} else {
					
					// Create the value
					field.FieldValues = append(field.FieldValues, models.FieldValue{FieldId: field.Id, Email: en, Value: vn})
					models.PutField(&field)
				}
			
				v := models.FieldValue{
					FieldId: field.Id,
					TargetId: target.Id,
					Email: en,
					Value: vn,
				}
				vs = append(vs, v)
			}
		}
	}
	return vs, nil
}

// CheckAndCreateSSL is a helper to setup self-signed certificates for the administrative interface.
func CheckAndCreateSSL(cp string, kp string) error {
	// Check whether there is an existing SSL certificate and/or key, and if so, abort execution of this function
	if _, err := os.Stat(cp); !os.IsNotExist(err) {
		return nil
	}
	if _, err := os.Stat(kp); !os.IsNotExist(err) {
		return nil
	}

	log.Infof("Creating new self-signed certificates for administration interface")

	priv, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)

	notBefore := time.Now()
	// Generate a certificate that lasts for 10 years
	notAfter := notBefore.Add(10 * 365 * 24 * time.Hour)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)

	if err != nil {
		return fmt.Errorf("TLS Certificate Generation: Failed to generate a random serial number: %s", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Gophish"},
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, priv.Public(), priv)
	if err != nil {
		return fmt.Errorf("TLS Certificate Generation: Failed to create certificate: %s", err)
	}

	certOut, err := os.Create(cp)
	if err != nil {
		return fmt.Errorf("TLS Certificate Generation: Failed to open %s for writing: %s", cp, err)
	}
	pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	certOut.Close()

	keyOut, err := os.OpenFile(kp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("TLS Certificate Generation: Failed to open %s for writing", kp)
	}

	b, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return fmt.Errorf("TLS Certificate Generation: Unable to marshal ECDSA private key: %v", err)
	}

	pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: b})
	keyOut.Close()

	log.Info("TLS Certificate Generation complete")
	return nil
}

// GenerateSecureKey creates a secure key to use as an API key
func GenerateSecureKey() string {
	// Inspired from gorilla/securecookie
	k := make([]byte, 32)
	io.ReadFull(rand.Reader, k)
	return fmt.Sprintf("%x", k)
}

// NewHash hashes the provided password and returns the bcrypt hash (using the
// default 10 rounds) as a string.
func NewHash(pass string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}
