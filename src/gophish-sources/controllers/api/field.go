package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	ctx "github.com/gophish/gophish/context"
	log "github.com/gophish/gophish/logger"
	"github.com/gophish/gophish/models"
	"github.com/gorilla/mux"
	"github.com/jinzhu/gorm"
)

// Fields returns a list of fields if requested via GET.
// If requested via POST, APIFields creates a new field and returns a reference to it.
func (as *Server) Fields(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == "GET":
		fs, err := models.GetFields(ctx.Get(r, "user_id").(int64))
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "No fields found"}, http.StatusNotFound)
			return
		}
		JSONResponse(w, fs, http.StatusOK)
	//POST: Create a new field and return it as JSON
	case r.Method == "POST":
		f := models.Field{}
		// Put the request into a field
		err := json.NewDecoder(r.Body).Decode(&f)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Invalid JSON structure"}, http.StatusBadRequest)
			return
		}
		_, err = models.GetFieldByName(f.Name, ctx.Get(r, "user_id").(int64))
		if err != gorm.ErrRecordNotFound {
			JSONResponse(w, models.Response{Success: false, Message: "Field name already in use"}, http.StatusConflict)
			return
		}
		f.ModifiedDate = time.Now().UTC()
		f.UserId = ctx.Get(r, "user_id").(int64)
		err = models.PostField(&f)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
			return
		}
		JSONResponse(w, f, http.StatusCreated)
	}
}

// FieldsSummary returns a summary of the fields owned by the current user.
func (as *Server) FieldsSummary(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == "GET":
		fs, err := models.GetFieldSummaries(ctx.Get(r, "user_id").(int64))
		if err != nil {
			log.Error(err)
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, fs, http.StatusOK)
	}
}

// Field returns details about the requested field.
// If the field is not valid, Field returns null.
func (as *Server) Field(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 0, 64)
	f, err := models.GetField(id, ctx.Get(r, "user_id").(int64))
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Field not found"}, http.StatusNotFound)
		return
	}
	switch {
	case r.Method == "GET":
		JSONResponse(w, f, http.StatusOK)
	case r.Method == "DELETE":
		err = models.DeleteField(&f)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Error deleting field"}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, models.Response{Success: true, Message: "Field deleted successfully!"}, http.StatusOK)
	case r.Method == "PUT":
		// Change this to get from URL and uid (don't bother with id in r.Body)
		f = models.Field{}
		err = json.NewDecoder(r.Body).Decode(&f)
		if f.Id != id {
			JSONResponse(w, models.Response{Success: false, Message: "Error: /:id and field_id mismatch"}, http.StatusInternalServerError)
			return
		}
		f.ModifiedDate = time.Now().UTC()
		f.UserId = ctx.Get(r, "user_id").(int64)
		err = models.PutField(&f)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
			return
		}
		JSONResponse(w, f, http.StatusOK)
	}
}

// FieldSummary returns a summary of the fields owned by the current user.
func (as *Server) FieldSummary(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == "GET":
		vars := mux.Vars(r)
		id, _ := strconv.ParseInt(vars["id"], 0, 64)
		f, err := models.GetFieldSummary(id, ctx.Get(r, "user_id").(int64))
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Field not found"}, http.StatusNotFound)
			return
		}
		JSONResponse(w, f, http.StatusOK)
	}
}
