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

// Variables returns a list of variables if requested via GET.
// If requested via POST, APIVariables creates a new variable and returns a reference to it.
func (as *Server) Variables(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == "GET":
		vs, err := models.GetVariables(ctx.Get(r, "user_id").(int64))
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "No variables found"}, http.StatusNotFound)
			return
		}
		JSONResponse(w, vs, http.StatusOK)
	//POST: Create a new variable and return it as JSON
	case r.Method == "POST":
		v := models.FrontVariable{}
		// Put the request into a variable
		err := json.NewDecoder(r.Body).Decode(&v)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Invalid JSON structure"}, http.StatusBadRequest)
			return
		}
		_, err = models.GetVariableByName(v.Name, ctx.Get(r, "user_id").(int64))
		if err != gorm.ErrRecordNotFound {
			JSONResponse(w, models.Response{Success: false, Message: "Variable name already in use"}, http.StatusConflict)
			return
		}
		v.ModifiedDate = time.Now().UTC()
		v.UserId = ctx.Get(r, "user_id").(int64)
		err = models.PostVariable(&v)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
			return
		}
		JSONResponse(w, v, http.StatusCreated)
	}
}

// VariablesSummary returns a summary of the variables owned by the current user.
func (as *Server) VariablesSummary(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == "GET":
		vs, err := models.GetVariableSummaries(ctx.Get(r, "user_id").(int64))
		if err != nil {
			log.Error(err)
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, vs, http.StatusOK)
	}
}

// Variable returns details about the requested variable.
// If the variable is not valid, Variable returns null.
func (as *Server) Variable(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 0, 64)
	v, err := models.GetVariable(id, ctx.Get(r, "user_id").(int64))
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Variable not found"}, http.StatusNotFound)
		return
	}
	switch {
	case r.Method == "GET":
		JSONResponse(w, v, http.StatusOK)
	case r.Method == "DELETE":
		err = models.DeleteVariable(&v)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Error deleting variable"}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, models.Response{Success: true, Message: "Variable deleted successfully!"}, http.StatusOK)
	case r.Method == "PUT":
		// Change this to get from URL and uid (don't bother with id in r.Body)
		fv := models.FrontVariable{}
		err = json.NewDecoder(r.Body).Decode(&fv)
		if fv.Id != id {
			JSONResponse(w, models.Response{Success: false, Message: "Error: /:id and variable_id mismatch"}, http.StatusInternalServerError)
			return
		}
		fv.ModifiedDate = time.Now().UTC()
		fv.UserId = ctx.Get(r, "user_id").(int64)
		err = models.PutVariable(&fv)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
			return
		}
		JSONResponse(w, fv, http.StatusOK)
	}
}

// VariableSummary returns a summary of the variables owned by the current user.
func (as *Server) VariableSummary(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == "GET":
		vars := mux.Vars(r)
		id, _ := strconv.ParseInt(vars["id"], 0, 64)
		v, err := models.GetVariableSummary(id, ctx.Get(r, "user_id").(int64))
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Variable not found"}, http.StatusNotFound)
			return
		}
		JSONResponse(w, v, http.StatusOK)
	}
}
