package models

import (
	"errors"
	"time"
	"strings"

	log "github.com/gophish/gophish/logger"
	"github.com/jinzhu/gorm"
	"github.com/sirupsen/logrus"
)

// Variable contains the variables needed for a user -> variable mapping
// Variable contain 1..* Conditions
type Variable struct {
	Id           int64     		`json:"id"`
	FieldId      int64     		`json:"-"`
	UserId       int64     		`json:"-"`
	Name         string    		`json:"name"`
	ModifiedDate time.Time 		`json:"modified_date"`
	Conditions   []Condition    `json:"conditions" sql:"-"`
}

// FrontVariable contains the variables needed for a user -> variable mapping
// FrontVariable contain 1..* Conditions
type FrontVariable struct {
	Id           int64     		`json:"id"`
	Type		 string			`json:"type"`
	FieldId      int64     		`json:"-"`
	Field		 string			`json:"field"`
	UserId       int64     		`json:"-"`
	Name         string    		`json:"name"`
	ModifiedDate time.Time 		`json:"modified_date"`
	Conditions   []Condition    `json:"conditions" sql:"-"`
}

// VariableSummaries is a struct representing the overview of Variables.
type VariableSummaries struct {
	Total  		int64          		`json:"total"`
	Variables   []VariableSummary   `json:"variables"`
}

// VariableSummary represents a summary of the Variable model. The only
// difference is that, instead of listing the COnditions (which could be expensive
// for large variables), it lists the condition count.
type VariableSummary struct {
	Id             int64     `json:"id"`
	Name           string    `json:"name"`
	FieldId        int64     `json:"-"`
	Field		   string	 `json:"field"`
	ModifiedDate   time.Time `json:"modified_date"`
	NumConditions  int64     `json:"num_conditions"`
}

// Condition is used for a one-to-many relationship between 1 Variable and 1..* Conditions
type Condition struct {
	VariableId  int64 		`json:"-"`
	Condition 	string		`json:"condition"`
	Value       string		`json:"value"`
}

// IdStruct2 is used a basic Id value
type IdStruct2 struct {
	Id  	 int64 		`json:"-"`
}

// ErrVariableNameNotSpecified is thrown when a variable name is not specified
var ErrVariableNameNotSpecified = errors.New("Variable name not specified")

// ErrNoConditionsSpecified is thrown when no conditions are specified by the user
var ErrNoConditionsSpecified = errors.New("No conditions specified")

// Validate performs validation on a variable given by the user
func (v *FrontVariable) Validate() error {
	switch {
	case v.Name == "":
		return ErrVariableNameNotSpecified
	case len(v.Conditions) == 0:
		return ErrNoConditionsSpecified
	}
	v.Name = strings.ToLower(v.Name)
	return nil
}

// GetVariables returns the variables owned by the given user.
func GetVariables(uid int64) ([]FrontVariable, error) {
	vs := []FrontVariable{}
	query := db.Table("variables").Joins("left join fields f on variables.field_id = f.id").Where("variables.user_id=?", uid)
	err := query.Select("variables.id as id, variables.name as name, field_id, f.name as field, variables.modified_date as modified_date").Scan(&vs).Error
	if err != nil {
		log.Error(err)
		return vs, err
	}
	for i := range vs {
		vs[i].Conditions, err = GetConditions(vs[i].Id)
		if err != nil {
			log.Error(err)
		}
	}
	return vs, nil
}

// GetVariableSummaries returns the summaries for the variables
// created by the given uid.
func GetVariableSummaries(uid int64) (VariableSummaries, error) {
	vs := VariableSummaries{}
	query := db.Table("variables").Joins("left join fields f on variables.field_id = f.id").Where("variables.user_id=?", uid)
	err := query.Select("variables.id as id, variables.name as name, field_id, f.name as field, variables.modified_date as modified_date").Scan(&vs.Variables).Error
	if err != nil {
		log.Error(err)
		return vs, err
	}
	for i := range vs.Variables {
		query = db.Table("conditions").Where("variable_id=?", vs.Variables[i].Id)
		err = query.Count(&vs.Variables[i].NumConditions).Error
		if err != nil {
			return vs, err
		}
	}
	vs.Total = int64(len(vs.Variables))
	return vs, nil
}

// GetVariable returns the variable, if it exists, specified by the given id and user_id.
func GetVariable(id int64, uid int64) (FrontVariable, error) {
	v := FrontVariable{}
	query := db.Table("variables").Joins("left join fields f on variables.field_id = f.id").Where("variables.user_id=? and variables.id=?", uid, id)
	err := query.Select("variables.id as id, variables.name as name, field_id, f.name as field, variables.modified_date as modified_date").Find(&v).Error
	if err != nil {
		log.Error(err)
		return v, err
	}
	v.Conditions, err = GetConditions(v.Id)
	if err != nil {
		log.Error(err)
	}
	return v, nil
}

// GetVariableSummary returns the summary for the requested variable
func GetVariableSummary(id int64, uid int64) (VariableSummary, error) {
	v := VariableSummary{}
	query := db.Table("variables").Joins("left join fields f on variables.field_id = f.id").Where("variables.user_id=? and variable.id=?", uid, id)
	err := query.Select("variables.id as id, variables.name as name, field_id, f.name as field, variables.modified_date as modified_date").Scan(&v).Error
	if err != nil {
		log.Error(err)
		return v, err
	}
	query = db.Table("conditions").Where("variable_id=?", v.Id)
		err = query.Count(&v.NumConditions).Error
	if err != nil {
		return v, err
	}
	return v, nil
}

// GetVariableByName returns the variable, if it exists, specified by the given name and user_id.
func GetVariableByName(n string, uid int64) (FrontVariable, error) {
	v := FrontVariable{}
	query := db.Table("variables").Joins("left join fields f on variables.field_id = f.id").Where("variables.user_id=? and variables.name=?", uid, n)
	err := query.Select("variables.id as id, variables.name as name, field_id, f.name as field, variables.modified_date as modified_date").Scan(&v).Error
	if err != nil {
		log.Error(err)
		return v, err
	}
	v.Conditions, err = GetConditions(v.Id)
	if err != nil {
		log.Error(err)
	}
	return v, err
}

// PostVariable creates a new variable in the database.
func PostVariable(v *FrontVariable) error {
	if err := v.Validate(); err != nil {
		return err
	}
	// Insert the variable into the DB
	dbv := Variable{}
	dbv.Id = v.Id
	dbv.FieldId = GetFieldIdByName(v.Field)
	if (v.Type == "simple") && (dbv.FieldId == 0) {
		return errors.New("Target Field doesn't exist")
	}
	dbv.UserId = v.UserId
	dbv.Name = v.Name
	dbv.ModifiedDate = v.ModifiedDate
	tx := db.Begin()
	err := tx.Save(&Variable{Id: dbv.Id, FieldId: dbv.FieldId, UserId: dbv.UserId, Name: dbv.Name, ModifiedDate: dbv.ModifiedDate}).Error
	if err != nil {
		tx.Rollback()
		log.Error(err)
		return err
	}
	vid := IdStruct2{}
	err = tx.Table("variables").Select("id").Where("name=?", v.Name).Scan(&vid).Error
	dbv.Id = vid.Id
	for _, t := range v.Conditions {
		err = insertConditionIntoVariable(tx, t, vid.Id)
		if err != nil {
			log.Error(err)
			tx.Rollback()
			return err
		}
	}
	err = tx.Commit().Error
	if err != nil {
		log.Error(err)
		tx.Rollback()
		return err
	}
	return nil
}

// PutVariable updates the given variable if found in the database.
func PutVariable(v *FrontVariable) error {
	if err := v.Validate(); err != nil {
		return err
	}
	// Fetch variable's existing conditions from database.
	cs := []Condition{}
	cs, err := GetConditions(v.Id)
	if err != nil {
		log.WithFields(logrus.Fields{
			"variable_id": v.Id,
		}).Error("Error getting conditions from variable")
		return err
	}
	
	// Preload the caches
	cacheNew := make(map[string]string, len(v.Conditions))
	for _, c := range v.Conditions {
		cacheNew[c.Condition] = c.Condition
	}
	
	cacheExisting := make(map[string]string, len(cs))
	for _, c := range cs {
		cacheExisting[c.Condition] = c.Condition
	}

	dbv := Variable{}
	dbv.Id = v.Id
	dbv.FieldId = GetFieldIdByName(v.Field)
	if (v.Type == "simple") && (dbv.FieldId == 0) {
		return errors.New("Target Field doesn't exist")
	}
	dbv.UserId = v.UserId
	dbv.Name = v.Name
	dbv.ModifiedDate = v.ModifiedDate
	tx := db.Begin()
	// Check existing conditions, removing any that are no longer in the variable.
	for _, c := range cs {
		if _, ok := cacheNew[c.Condition]; ok {
			continue
		}
		// If the condition does not exist in the variable any longer, we delete it
		err := tx.Where("variable_id=? and condition=?", v.Id, c.Condition).Delete(&Condition{}).Error
		if err != nil {
			tx.Rollback()
			log.WithFields(logrus.Fields{
				"condition": c.Condition,
			}).Error("Error deleting condition")
		}
	}
	// Add any conditions that are not in the database yet.
	for _, nc := range v.Conditions {
		// If the condition already exists in the database, we don't do anything
		// the record with the latest information.
		if _, ok := cacheExisting[nc.Condition]; ok {
			continue
		}
		// Otherwise, add condition if not in database
		err = insertConditionIntoVariable(tx, nc, v.Id)
		if err != nil {
			log.Error(err)
			tx.Rollback()
			return err
		}
	}
	
	err = tx.Save(&Variable{Id: dbv.Id, FieldId: dbv.FieldId, UserId: dbv.UserId, Name: dbv.Name, ModifiedDate: dbv.ModifiedDate}).Error
	if err != nil {
		log.Error(err)
		return err
	}
	err = tx.Commit().Error
	if err != nil {
		tx.Rollback()
		return err
	}
	return nil
}

// DeleteVariable deletes a given variable by variable ID and user ID
func DeleteVariable(v *FrontVariable) error {
	// Delete all the conditions entries for this variable
	err := db.Where("variable_id=?", v.Id).Delete(&Condition{}).Error
	if err != nil {
		log.Error(err)
		return err
	}
	dbv := Variable{}
	dbv.Id = v.Id
	dbv.FieldId = GetFieldIdByName(v.Field)
	dbv.UserId = v.UserId
	dbv.Name = v.Name
	dbv.ModifiedDate = v.ModifiedDate
	// Delete the variable itself
	err = db.Delete(dbv).Error
	if err != nil {
		log.Error(err)
		return err
	}
	return err
}

func insertConditionIntoVariable(cx *gorm.DB, c Condition, vid int64) error {
	err := cx.Save(&Condition{VariableId: vid, Condition: c.Condition, Value: c.Value}).Error
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// GetConditions performs a one-to-many select to get all the conditions for a variable
func GetConditions(vid int64) ([]Condition, error) {
	cs := []Condition{}
	err := db.Table("conditions").Select("variable_id, condition, value").Where("variable_id=?", vid).Scan(&cs).Error
	return cs, err
}

// GetFieldIdByName returns the field id, if it exists, specified by the given name
func GetFieldIdByName(field string) int64 {
	fid := IdStruct2{}
	err := db.Table("fields").Select("id").Where("name=?", field).Scan(&fid).Error
	if err != nil {
		return 0
	}
	return fid.Id
}
