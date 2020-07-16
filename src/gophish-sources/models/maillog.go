package models

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"net/mail"
	"os"
	"strings"
	"time"
	"strconv"

	"github.com/gophish/gomail"
	"github.com/gophish/gophish/config"
	log "github.com/gophish/gophish/logger"
	"github.com/gophish/gophish/mailer"
)

// MaxSendAttempts set to 8 since we exponentially backoff after each failed send
// attempt. This will give us a maximum send delay of 256 minutes, or about 4.2 hours.
var MaxSendAttempts = 8

// ErrMaxSendAttempts is thrown when the maximum number of sending attempts for a given
// MailLog is exceeded.
var ErrMaxSendAttempts = errors.New("max send attempts exceeded")

// MailLog is a struct that holds information about an email that is to be
// sent out.
type MailLog struct {
	Id          int64     `json:"-"`
	UserId      int64     `json:"-"`
	CampaignId  int64     `json:"campaign_id"`
	RId         string    `json:"id"`
	SendDate    time.Time `json:"send_date"`
	SendAttempt int       `json:"send_attempt"`
	Processing  bool      `json:"-"`

	cachedCampaign *Campaign
}

// StructId is a struct that holds the id of something
type StructId struct {
	Id          string     `json:"-"`
}

// StructValue is a struct that holds the value of something
type StructValue struct {
	Value       string     `json:"-"`
}

// Var is a struct that holds the fields of a variable
type Var struct {
	Id          string     `json:"-"`
	FieldId     string     `json:"-"`
	Name        string     `json:"-"`
}

// Cond is a struct that holds the fields of a condition
type Cond struct {
	Condition   string     `json:"-"`
	Value   	string     `json:"-"`
}

// GenerateMailLog creates a new maillog for the given campaign and
// result. It sets the initial send date to match the campaign's launch date.
func GenerateMailLog(c *Campaign, r *Result, sendDate time.Time) error {
	m := &MailLog{
		UserId:     c.UserId,
		CampaignId: c.Id,
		RId:        r.RId,
		SendDate:   sendDate,
	}
	return db.Save(m).Error
}

// Backoff sets the MailLog SendDate to be the next entry in an exponential
// backoff. ErrMaxRetriesExceeded is thrown if this maillog has been retried
// too many times. Backoff also unlocks the maillog so that it can be processed
// again in the future.
func (m *MailLog) Backoff(reason error) error {
	r, err := GetResult(m.RId)
	if err != nil {
		return err
	}
	if m.SendAttempt == MaxSendAttempts {
		r.HandleEmailError(ErrMaxSendAttempts)
		return ErrMaxSendAttempts
	}
	// Add an error, since we had to backoff because of a
	// temporary error of some sort during the SMTP transaction
	m.SendAttempt++
	backoffDuration := math.Pow(2, float64(m.SendAttempt))
	m.SendDate = m.SendDate.Add(time.Minute * time.Duration(backoffDuration))
	err = db.Save(m).Error
	if err != nil {
		return err
	}
	err = r.HandleEmailBackoff(reason, m.SendDate)
	if err != nil {
		return err
	}
	err = m.Unlock()
	return err
}

// Unlock removes the processing flag so the maillog can be processed again
func (m *MailLog) Unlock() error {
	m.Processing = false
	return db.Save(&m).Error
}

// Lock sets the processing flag so that other processes cannot modify the maillog
func (m *MailLog) Lock() error {
	m.Processing = true
	return db.Save(&m).Error
}

// Error sets the error status on the models.Result that the
// maillog refers to. Since MailLog errors are permanent,
// this action also deletes the maillog.
func (m *MailLog) Error(e error) error {
	r, err := GetResult(m.RId)
	if err != nil {
		log.Warn(err)
		return err
	}
	err = r.HandleEmailError(e)
	if err != nil {
		log.Warn(err)
		return err
	}
	err = db.Delete(m).Error
	return err
}

// Success deletes the maillog from the database and updates the underlying
// campaign result.
func (m *MailLog) Success() error {
	r, err := GetResult(m.RId)
	if err != nil {
		return err
	}
	err = r.HandleEmailSent()
	if err != nil {
		return err
	}
	err = db.Delete(m).Error
	return nil
}

// GetDialer returns a dialer based on the maillog campaign's SMTP configuration
func (m *MailLog) GetDialer() (mailer.Dialer, error) {
	c := m.cachedCampaign
	if c == nil {
		campaign, err := GetCampaignMailContext(m.CampaignId, m.UserId)
		if err != nil {
			return nil, err
		}
		c = &campaign
	}
	return c.SMTP.GetDialer()
}

// CacheCampaign allows bulk-mail workers to cache the otherwise expensive
// campaign lookup operation by providing a pointer to the campaign here.
func (m *MailLog) CacheCampaign(campaign *Campaign) error {
	if campaign.Id != m.CampaignId {
		return fmt.Errorf("incorrect campaign provided for caching. expected %d got %d", m.CampaignId, campaign.Id)
	}
	m.cachedCampaign = campaign
	return nil
}

// Generate fills in the details of a gomail.Message instance with
// the correct headers and body from the campaign and recipient listed in
// the maillog. We accept the gomail.Message as an argument so that the caller
// can choose to re-use the message across recipients.
func (m *MailLog) Generate(msg *gomail.Message) error {
	r, err := GetResult(m.RId)
	if err != nil {
		return err
	}
	c := m.cachedCampaign
	if c == nil {
		campaign, err := GetCampaignMailContext(m.CampaignId, m.UserId)
		if err != nil {
			return err
		}
		c = &campaign
	}

	f, err := mail.ParseAddress(c.SMTP.FromAddress)
	if err != nil {
		return err
	}
	msg.SetAddressHeader("From", f.Address, f.Name)

	ptx, err := NewPhishingTemplateContext(c, r.BaseRecipient, r.RId)
	if err != nil {
		return err
	}

	// Add the transparency headers
	msg.SetHeader("X-Mailer", config.ServerName)
	if conf.ContactAddress != "" {
		msg.SetHeader("X-Gophish-Contact", conf.ContactAddress)
	}

	// Add Message-Id header as described in RFC 2822.
	messageID, err := m.generateMessageID()
	if err != nil {
		return err
	}
	msg.SetHeader("Message-Id", messageID)

	// Parse the customHeader templates
	for _, header := range c.SMTP.Headers {
		key, err := ExecuteTemplate(header.Key, ptx)
		if err != nil {
			log.Warn(err)
		}

		value, err := ExecuteTemplate(header.Value, ptx)
		if err != nil {
			log.Warn(err)
		}

		// Add our header immediately
		msg.SetHeader(key, value)
	}

	// Parse remaining templates
	subject, err := ExecuteTemplate(c.Template.Subject, ptx)
	if err != nil {
		log.Warn(err)
	}
	// don't set Subject header if the subject is empty
	if len(subject) != 0 {
		subject, err = ReplaceReferences(subject, r.BaseRecipient.Email)
		msg.SetHeader("Subject", subject)
	}

	msg.SetHeader("To", r.FormatAddress())
	if c.Template.Text != "" {
		text, err := ExecuteTemplate(c.Template.Text, ptx)
		if err != nil {
			log.Warn(err)
		}
		text, err = ReplaceReferences(text, r.BaseRecipient.Email)
		if err != nil {
			log.Error(err)
		}
		msg.SetBody("text/plain", text)
	}
	if c.Template.HTML != "" {
		html, err := ExecuteTemplate(c.Template.HTML, ptx)
		if err != nil {
			log.Warn(err)
		}
		html, err = ReplaceReferences(html, r.BaseRecipient.Email)
		if err != nil {
			log.Error(err)
		}
		if c.Template.Text == "" {
			msg.SetBody("text/html", html)
		} else {
			msg.AddAlternative("text/html", html)
		}
	}
	// Attach the files
	for _, a := range c.Template.Attachments {
		msg.Attach(func(a Attachment) (string, gomail.FileSetting, gomail.FileSetting) {
			h := map[string][]string{"Content-ID": {fmt.Sprintf("<%s>", a.Name)}}
			return a.Name, gomail.SetCopyFunc(func(w io.Writer) error {
				decoder := base64.NewDecoder(base64.StdEncoding, strings.NewReader(a.Content))
				_, err = io.Copy(w, decoder)
				return err
			}), gomail.SetHeader(h)
		}(a))
	}

	return nil
}

// GetQueuedMailLogs returns the mail logs that are queued up for the given minute.
func GetQueuedMailLogs(t time.Time) ([]*MailLog, error) {
	ms := []*MailLog{}
	err := db.Where("send_date <= ? AND processing = ?", t, false).
		Find(&ms).Error
	if err != nil {
		log.Warn(err)
	}
	return ms, err
}

// GetMailLogsByCampaign returns all of the mail logs for a given campaign.
func GetMailLogsByCampaign(cid int64) ([]*MailLog, error) {
	ms := []*MailLog{}
	err := db.Where("campaign_id = ?", cid).Find(&ms).Error
	return ms, err
}

// LockMailLogs locks or unlocks a slice of maillogs for processing.
func LockMailLogs(ms []*MailLog, lock bool) error {
	tx := db.Begin()
	for i := range ms {
		ms[i].Processing = lock
		err := tx.Save(ms[i]).Error
		if err != nil {
			tx.Rollback()
			return err
		}
	}
	tx.Commit()
	return nil
}

// UnlockAllMailLogs removes the processing lock for all maillogs
// in the database. This is intended to be called when Gophish is started
// so that any previously locked maillogs can resume processing.
func UnlockAllMailLogs() error {
	return db.Model(&MailLog{}).Update("processing", false).Error
}

var maxBigInt = big.NewInt(math.MaxInt64)

// generateMessageID generates and returns a string suitable for an RFC 2822
// compliant Message-ID, e.g.:
// <1444789264909237300.3464.1819418242800517193@DESKTOP01>
//
// The following parameters are used to generate a Message-ID:
// - The nanoseconds since Epoch
// - The calling PID
// - A cryptographically random int64
// - The sending hostname
func (m *MailLog) generateMessageID() (string, error) {
	t := time.Now().UnixNano()
	pid := os.Getpid()
	rint, err := rand.Int(rand.Reader, maxBigInt)
	if err != nil {
		return "", err
	}
	h, err := os.Hostname()
	// If we can't get the hostname, we'll use localhost
	if err != nil {
		h = "localhost.localdomain"
	}
	msgid := fmt.Sprintf("<%d.%d.%d@%s>", t, pid, rint, h)
	return msgid, nil
}

// ReplaceReferences replace all the references of a target field or variable created in the database
func ReplaceReferences(text string, target_email string) (string, error) {

	// Retrieve target id for getting the value of fields
	uid := StructId{}
	query := db.Table("targets").Where("email=?", target_email)
	err := query.Select("id").First(&uid).Error
	if err != nil {
		log.Error(err)
		return text, err
	}
	
	// Start a loop in the text looking for a block
	for i := 0; i < len(text); i++ {
		if string(text[i]) == "{" {
			text, i = ReplaceBlock(text, i, uid.Id)
		}
	}
	
	return text, nil
}

// ReplaceBlock returns the evaluation of the block condition
func ReplaceBlock(text string, start_block int, uid string) (string, int) {

	returned_i := start_block
	
	// If the block is a simple reference name
	if string(text[start_block+1]) == "%" {
		text, returned_i = ReplaceReference(text, uid, start_block)
		
	// Else if the block is a reference condition
	} else if string(text[start_block+1:start_block+5]) == "if(%" {
		
		buff_value := ""
		reference := ""
		value := ""
		append_text := ""
		end_block := 0
		start_cond := start_block+5
		match_index := 0
		end_cond := 0
		start_if := 0
		end_if := 0
		start_else := 0
		end_else := 0
		start_nested := 0
		end_nested := 0
		
		// Find the end of the reference name
		match_index = FindChars(text, "%)", start_cond)
		if match_index > start_cond {
			
			end_cond = match_index
			
			// The reference name is between '%' and '%'
			reference = text[start_cond:end_cond]
			text = strings.Replace(text, string(text[(start_block+3):end_cond+2]), "($" + reference + "$)", 1)
			
			if ! strings.Contains(reference, " ") {
				// Find the value of that reference for that user, if it exists
				ref_value, err := GetFieldValue(uid, strings.ToLower(reference))
				if err != nil {
					ref_value = ""
				}
				if ref_value == "" {
					// Find the value of that variable for that user, if it exists
					ref_value, err = GetVariableValue(uid, strings.ToLower(reference))
				}
				buff_value = ref_value
				value = ref_value
				
				// Find the start of the if statement
				match_index = FindChars(text, "{", (end_cond+2))
				if match_index > (end_cond+1) {
					start_if = match_index
				}
				
			}
		}
		
		if start_if > 0 {
		
			// If there is a nested if, replace the block
			if string(text[start_if:start_if+7]) == "{ {if(%" {
				text = strings.Replace(text, string(text[start_if:start_if+7]), "{{if(%", 1)
				start_nested = start_if+1
				text, end_nested = ReplaceBlock(text, start_nested, uid)
				
				// Find the end of the if statement, replacing all the sub references
				text, match_index, buff_value = ReplaceSubReferences(text, uid, end_nested+1, reference, value, buff_value)
				
			} else {
				// Find the end of the if statement, replacing all the sub references
				text, match_index, buff_value = ReplaceSubReferences(text, uid, start_if+1, reference, value, buff_value)
			}
			
			if match_index > start_if {
				end_if = match_index
			}
		}
		
		if end_if > 0 {
			
			// Find the start of the else statement
			match_index = FindChars(text, "{", (end_if+1))
			if match_index > end_if {
				start_else = match_index
			}
		}
		
		if start_else > 0 {
			
			// If there is a nested else, replace the block
			if string(text[start_else:start_else+7]) == "{ {if(%" {
			
				text = strings.Replace(text, string(text[start_else:start_else+7]), "{{if(%", 1)
				start_nested = start_else+1
				text, end_nested = ReplaceBlock(text, start_nested, uid)
				
				// Find the end of the if statement, replacing all the sub references
				text, match_index, buff_value = ReplaceSubReferences(text, uid, end_nested+1, reference, value, buff_value)
				
			} else {
				// Find the end of the if statement, replacing all the sub references
				text, match_index, buff_value = ReplaceSubReferences(text, uid, start_else+1, reference, value, buff_value)
			}
			
			if match_index > start_else {
				end_else = match_index
			}
		}
		
		if end_else > 0 {
		
			// Find the end of the block
			match_index = FindChars(text, "}", (end_else+1))
			
			if match_index > end_else {
				end_block = match_index
			}
		}
		
		if end_block > 0 {
			
			// Append the right text and return text and updated index
			if value != "" {
				if end_if - start_if > 1 {
					append_text = text[start_if+1:end_if]
				} else {
					append_text = ""
				}
			} else {
			
				if end_else - start_else > 1 {
					append_text = text[start_else+1:end_else]
				} else {
					append_text = ""
				}
			}
			
			text = strings.Replace(text, text[start_block:end_block+1], append_text, 1)
			returned_i = start_block + len(append_text) - 1

			return text, returned_i
		}
	}
	return text, returned_i
}

// FindChars return the position of the first chars matching
// in the given string, or the given starting index if not found  
func FindChars(text string, match string, start_search int) int {
	
	for i := start_search; i < len(text); i++ {
		if i+len(match) <= len(text) {
			if string(text[i:(i+len(match))]) == match {
				return i
			}
		}
	}
	return start_search
}

// ReplaceSubReferences return the position of the end of the statement,
// after replaced all the included references, if there are any  
func ReplaceSubReferences(text string, uid string, start_search int, reference string, value string, buff_value string) (string, int, string) {

	for i := start_search; i < len(text); i++ {
		if string(text[i]) == "%" {
			text, buff_value, i = ReplaceSubReference(text, uid, i, reference, value, buff_value)
			
		} else if string(text[i]) == "}" {
			return text, i, buff_value
		}
	}
	return text, start_search, buff_value
}

// ReplaceReference finds the reference name within the condition 
// and evaluate it to know if the value is empty or not
func ReplaceReference(text string, uid string, start_ref int) (string, int) {

	returned_i := start_ref
	
	// Start a loop to find the end of the name
	for r := start_ref+2; r < len(text); r++ {
	
		// If the end of the block is found ('%')
		if string(text[r:r+2]) == "%}" {
		
			// The reference name is between '%' and '%'
			reference := text[(start_ref+2):(r)]
			if ! strings.Contains(reference, " ") {
			
				// Find the value of that reference for that user, if it exists
				value, err := GetFieldValue(uid, strings.ToLower(reference))
				if err != nil {
					value = ""
				}
				if value == "" {
					// Find the value of that variable for that user, if it exists
					value, err = GetVariableValue(uid, strings.ToLower(reference))
				}
				
				// Replace the reference with the value and rearrange the index
				text = strings.Replace(text, text[start_ref:r+2], value, 1)
				returned_i = start_ref + len(value) - 1
				
			} else {
				returned_i = r
			}
			break
		}
	}
	return text, returned_i
}

// ReplaceSubReference finds the reference name within a 'if' or 'else' statement 
// and replaces it with the relative value
func ReplaceSubReference(text string, uid string, start_ref int, reference string, value string, buff_value string) (string, string, int) {

	returned_i := start_ref
	
	// If it is a shortcut reference
	if string(text[start_ref+1]) == "%" {
		
		text = strings.Replace(text, text[start_ref:start_ref+2], buff_value, 1)
		returned_i = start_ref + len(buff_value) - 1
		
	// Else if it is a reference name
	} else {
	
		// Start a loop to find the end of the name
		for sb := start_ref+1; sb < len(text); sb++ {
		
			// If the end of the reference is found ('%')
			if string(text[sb]) == "%" {
			
				// The reference name is between '%' and '%'
				end_field := sb
				tmp_field := text[(start_ref+1):end_field]
				
				// If the reference name is different from the condition one
				if tmp_field != reference {
				
					if ! strings.Contains(tmp_field, " ") {
					
						// Find the value of that sub-reference for that user, if it exists
						tmp_value, err := GetFieldValue(uid, strings.ToLower(tmp_field))
						if err != nil {
							tmp_value = ""
						}
						if tmp_value == "" {
							// Find the value of that variable for that user, if it exists
							tmp_value, err = GetVariableValue(uid, strings.ToLower(tmp_field))
						}
						buff_value = tmp_value
						
						// Replace the reference with the value and rearrange the index
						text = strings.Replace(text, text[start_ref:end_field+1], tmp_value, 1)
						returned_i = start_ref + len(tmp_value) - 1
						
					} else {
						returned_i = end_field
					}
					break
					
				} else {
					// Change the structure of the condition reference to avoid replacing the 
					// wrong text, then replace with the value and replace again the initial condition
					text = strings.Replace(text, text[start_ref:end_field+1], value, 1)
					returned_i = start_ref + len(value) - 1
					break
				}
			}
		}
	}
	return text, buff_value, returned_i
}

// GetFieldValue returns the value of a certain field respect to a certain target
func GetFieldValue(uid string, field string) (string, error) {

	// Retrieve the value of that field respect to the given target
	value := StructValue{}
	query := db.Table("fields").Select("tf.value").Joins("left join target_fields tf ON fields.id = tf.field_id")
	err := query.Where("fields.name=? and tf.target_id=?", field, uid).First(&value).Error
	if err != nil {
		return "", err
	}
	return value.Value, nil
}

// GetVariableValue returns the value of a certain variable 
// respect to a certain target, based on the type of condition
func GetVariableValue(uid string, variable string) (string, error) {

	// Retrieve variable information for getting the condition value for the specific target
	var_type := Var{}
	value := StructValue{}
	query := db.Table("variables").Select("id, field_id, name").Where("name=?", variable)
	err := query.First(&var_type).Error
	if err != nil {
		log.Error(err)
		return "", err
	}
	
	// If the field exists
	if var_type.FieldId != "" {
	
		// If the condition is simple
		if var_type.FieldId != "0" {
			// Retrieve condition value for the specific target and field
			query := db.Table("conditions").Select("conditions.value as value").Joins("left join target_fields tf ON conditions.condition = tf.value")
			err := query.Where("tf.field_id=? and tf.target_id=? and conditions.variable_id=?", var_type.FieldId, uid, var_type.Id).First(&value).Error
			if err != nil {
				return "", err
			}
			return value.Value, nil
			
		// Else if the condition is complex
		} else {
		
			conditions := []Cond{}
			err := db.Table("conditions").Select("condition, value").Where("variable_id=?", var_type.Id).Scan(&conditions).Error
			if err != nil {
				return "", err
			}
			
			match := false
			for _, c := range conditions {
			
				if (strings.Count(c.Condition, "(")) == (strings.Count(c.Condition, ")")) {
					condition := ReplaceValueSpaces(c.Condition)
					condition = RemoveSpaces(condition)
					
					match, err = SplitComplexCondition(condition, uid)
					if err != nil {
						return "", err
					}
					if match == true {
						return c.Value, nil
					}
				}
			}
			return "", nil
		}
	}
	return "", nil
}

// RemoveSpaces removes all needless spaces of a condition
func RemoveSpaces(condition string) string {

	cond := strings.ReplaceAll(condition, "  ", " ")
	cond = strings.ReplaceAll(cond, "( ", "(")
	cond = strings.ReplaceAll(cond, " )", ")")
	return cond
}

// ReplaceValueSpaces replaces all spaces of a value into the url encoding symbol
func ReplaceValueSpaces(condition string) string {

	for i := 0; i < len(condition); i++ {
	
		// If the start of a value is found ('\"') that is not empty
		if string(condition[i]) == "\"" {
		
			if string(condition[i+1]) != "\"" {
			
				for j := i+1; j < len(condition); j++ {
				
					// If the end of a value is found ('\"')
					if string(condition[j]) == "\"" {
					
						// Replace all spaces with "%20" symbol and reset the index
						count_spaces := strings.Count(string(condition[i:j+1]), " ")
						condition = condition[0:i] + strings.ReplaceAll(condition[i:j+1], " ", "%20") + condition[j+1:]
						if count_spaces == 0 {
							i = j
						} else {
							i = j + (2 * count_spaces)
						}
						break
					}
				}
				
			} else {
				// If the value is empty move to the next char
				i++
			}
		}
	}
	return condition
}

// SplitComplexCondition splites the complex condition 
// for solving them one by one
func SplitComplexCondition(condition string, uid string) (bool, error) {

	// If the conditions have the same priority
	if strings.Count(condition, "(") == 0 {
	
		// Split the elements and evaluate what is not "true" "false", or empty
		elements := strings.Split(condition, " ")
		
		for i := 0; i < len(elements); i++ {
		
			e := elements[i]
			if (e != "true") && (e != "false") && (e != "") && (strings.ToLower(e) != "and") && (strings.ToLower(e) != "or") {
			
				field := e
				operator := elements[i+1]
				value := elements[i+2]
				elements[i] = strconv.FormatBool(MatchCondition(field, operator, value, uid))
				for j := i+3; j < len(elements); j++ {
					elements[j-2] = elements[j]
				}
				elements = elements[:len(elements)-2]
			}
		}
		return EvaluateExpression(elements)
		
	} else {
		stack := []string{}
		// Loop the condition to find blocks of expressions
		for i := 0; i < len(condition); i++ {
		
			// If the start of a block is found, push the index in the stack
			if string(condition[i]) == "(" {
				stack = append(stack, strconv.Itoa(i))
				
			// Else if the end of a block is found, pop the last index in the stack and
			// call recursively the function to replace it with a boolean value
			} else if string(condition[i]) == ")" {
			
				start_block, err := strconv.Atoi(stack[len(stack) - 1])
				end_block := i
				result, err := SplitComplexCondition(condition[start_block+1:end_block], uid)
				replacement := ""
				if err != nil {
					replacement = "false"
				} else {
					replacement = strconv.FormatBool(result)
				}
				
				condition = condition[0:start_block] + replacement + condition[end_block+1:]
				stack = stack[:len(stack) - 1]
				
				// Reset the index to continue the loop
				if replacement == "true" {
					i = start_block + 3
				} else {
					i = start_block + 4
				}
			}
		}
		
		if len(stack) > 0 {
			return false, nil
		}
		return SplitComplexCondition(condition, uid)
	}
}

// MatchCondition tries to match the single condition
// with respect to the vale of the field for the specific target
func MatchCondition(field string, operator string, value string, uid string) (bool) {

	// Replace the original spaces and quotes of the value
	value = strings.ReplaceAll(value, "%20", " ")
	value = strings.ReplaceAll(value, "\"", "")
	
	// Retrieve the value of that field respect to the given target
	target_value := StructValue{}
	query := db.Table("fields").Select("tf.value").Joins("left join target_fields tf ON fields.id = tf.field_id")
	err := query.Where("fields.name=? and tf.target_id=?", strings.ToLower(field), uid).First(&target_value).Error
	if err != nil {
		return false
	}
	
	// if the value has to be equal and it is, or if the value has to be not equal and it is, then the condition matches
	if ((target_value.Value == value) && (operator == "==")) || ((target_value.Value != value) && (operator == "!=")) {
		return true
	} else {
		return false
	}
}

// EvaluateExpression evaluates the boolean espession to return the result
// with respect to the vale of the field for the specific target
func EvaluateExpression(elements []string) (bool, error) {

	for len(elements) > 1 {
		value1 := elements[0]
		operator := elements[1]
		value2 := elements[2]
		
		// Evaluate the expression
		if strings.ToLower(operator) == "and" {
			if (value1 == "true") && (value2 == "true") {
				elements[0] = "true"
			} else {
				elements[0] = "false"
			}
		} else if strings.ToLower(operator) == "or" {
			if (value1 == "true") || (value2 == "true") {
				elements[0] = "true"
			} else {
				elements[0] = "false"
			}
		} else {
			elements[0] = "false"
		}
		
		// Delete elements 1 and 2 in order to start again
		for j := 3; j < len(elements); j++ {
			elements[j-2] = elements[j]
		}
		elements = elements[:len(elements)-2]
	}
	if elements[0] == "true" {
		return true, nil
	} else {
		return false, nil
	}
}