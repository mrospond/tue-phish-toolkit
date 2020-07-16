var variables = []

// Change the value of the variable type
var variableType = 0;
function changeType(type) {
    variableType = type.value;
	if (variableType == "simple") {
		document.getElementById("field").style.display = "block";
		document.getElementById("field").required = true;
		document.getElementById("fieldLabel").style.display = "block";
	} else {
		document.getElementById("field").style.display = "none";
		document.getElementById("field").required = false;
		document.getElementById("fieldLabel").style.display = "none";
		document.getElementById("field").value = ""
	}
}

// Save attempts to POST or PUT to /variables/
function save(id) {
    var conditions = []
    $.each($("#conditionsTable").DataTable().rows().data(), function (i, condition) {
        conditions.push({
            condition: unescapeHtml(condition[0]),
            value: unescapeHtml(condition[1])
        })
    })
    var variable = {
        name: $("#name").val(),
		type: $('input[name=type]:checked').val(),
        field: $("#field").val(),
        conditions: conditions
    }
    // Submit the variable
    if (id != -1) {
        // If we're just editing an existing variable,
        // we need to PUT /variables/:id
        variable.id = id
        api.variableId.put(variable)
            .success(function (data) {
                successFlash("Variable updated successfully!")
                load()
                dismiss()
                $("#modal").modal('hide')
            })
            .error(function (data) {
                modalError(data.responseJSON.message)
            })
    } else {
        // Else, if this is a new variable, POST it
        // to /variables
        api.variables.post(variable)
            .success(function (data) {
                successFlash("Variable added successfully!")
                load()
                dismiss()
                $("#modal").modal('hide')
            })
            .error(function (data) {
                modalError(data.responseJSON.message)
            })
    }
}

function dismiss() {
    $("#conditionsTable").dataTable().DataTable().clear().draw()
    $("#name").val("")
    $("#modal\\.flashes").empty()
}

function edit(id) {
    conditions = $("#conditionsTable").dataTable({
        destroy: true, // Destroy any other instantiated table - http://datatables.net/manual/tech-notes/3#destroy
        columnDefs: [{
            orderable: false,
            targets: "no-sort"
        }]
    })
    $("#modalSubmit").unbind('click').click(function () {
        save(id)
    })
    if (id == -1) {
        var variable = {}
    } else {
        api.variableId.get(id)
            .success(function (variable) {
				if (variable.field == "") {
					document.getElementById("field").style.display = "none";
					document.getElementById("field").required = false;
					document.getElementById("fieldLabel").style.display = "none";
					document.getElementById("simple").checked = false;
					document.getElementById("complex").checked = true;
				}
                $("#name").val(variable.name)
                $("#field").val(variable.field)
                $.each(variable.conditions, function (i, record) {
                    conditions.DataTable()
                        .row.add([
                            escapeHtml(record.condition),
                            escapeHtml(record.value),
                            '<span style="cursor:pointer;"><i class="fa fa-trash-o"></i></span>'
                        ]).draw()
                });

            })
            .error(function () {
                errorFlash("Error fetching variable")
            })
    }
    // Handle file uploads
    $("#csvconditionsupload").fileupload({
        url: "/api/import/variable",
        dataType: "json",
        beforeSend: function (xhr) {
            xhr.setRequestHeader('Authorization', 'Bearer ' + user.api_key);
        },
        add: function (e, data) {
            $("#modal\\.flashes").empty()
            var acceptFileTypes = /(csv|txt)$/i;
            var filename = data.originalFiles[0]['name']
            if (filename && !acceptFileTypes.test(filename.split(".").pop())) {
                modalError("Unsupported file extension (use .csv or .txt)")
                return false;
            }
            data.submit();
        },
        done: function (e, data) {
            $.each(data.result, function (i, record) {
                addCondition(
                    record.condition,
                    record.value);
            });
            conditions.DataTable().draw();
        }
    })
}

var downloadCSVTemplate = function () {
    var csvScope = [{
        'Condition': 'Example',
        'Value': 'Example Text'
    }]
    var filename = 'variable_template.csv'
    var csvString = Papa.unparse(csvScope, {})
    var csvData = new Blob([csvString], {
        type: 'text/csv;charset=utf-8;'
    });
    if (navigator.msSaveBlob) {
        navigator.msSaveBlob(csvData, filename);
    } else {
        var csvURL = window.URL.createObjectURL(csvData);
        var dlLink = document.createElement('a');
        dlLink.href = csvURL;
        dlLink.setAttribute('download', filename)
        document.body.appendChild(dlLink)
        dlLink.click();
        document.body.removeChild(dlLink)
    }
}

var deleteVariable = function (id) {
    var variable = variables.find(function (x) {
        return x.id === id
    })
    if (!variable) {
        return
    }
    Swal.fire({
        title: "Are you sure?",
        text: "This will delete the variable. This can't be undone!",
        type: "warning",
        animation: false,
        showCancelButton: true,
        confirmButtonText: "Delete " + escapeHtml(variable.name),
        confirmButtonColor: "#428bca",
        reverseButtons: true,
        allowOutsideClick: false,
        preConfirm: function () {
            return new Promise(function (resolve, reject) {
                api.variableId.delete(id)
                    .success(function (msg) {
                        resolve()
                    })
                    .error(function (data) {
                        reject(data.responseJSON.message)
                    })
            })
        }
    }).then(function (result) {
        if (result.value){
            Swal.fire(
                'Variable Deleted!',
                'This variable has been deleted!',
                'success'
            );
        }
        $('button:contains("OK")').on('click', function () {
            location.reload()
        })
    })
}

function addCondition(conditionInput, valueInput) {
    // Create new data row.
    var newRow = [
        escapeHtml(conditionInput),
        escapeHtml(valueInput),
        '<span style="cursor:pointer;"><i class="fa fa-trash-o"></i></span>'
    ];

    // Check table to see if condition already exists.
    var conditionsTable = conditions.DataTable();
    var existingRowIndex = conditionsTable
        .column(0, {
            order: "index"
        }) // Condition column has index of 0
        .data()
        .indexOf(condition);
    // Update or add new row as necessary.
    if (existingRowIndex >= 0) {
        conditionsTable
            .row(existingRowIndex, {
                order: "index"
            })
            .data(newRow);
    } else {
        conditionsTable.row.add(newRow);
    }
}

function load() {
    $("#variableTable").hide()
    $("#emptyMessage").hide()
    $("#loading").show()
    api.variables.summary()
        .success(function (response) {
            $("#loading").hide()
            if (response.total > 0) {
                variables = response.variables
                $("#emptyMessage").hide()
                $("#variableTable").show()
                var variableTable = $("#variableTable").DataTable({
                    destroy: true,
                    columnDefs: [{
                        orderable: false,
                        targets: "no-sort"
                    }]
                });
                variableTable.clear();
                $.each(variables, function (i, variable) {
                    variableTable.row.add([
                        escapeHtml(variable.name),
                        escapeHtml(variable.field),
                        escapeHtml(variable.num_conditions),
                        moment(variable.modified_date).format('MMMM Do YYYY, h:mm:ss a'),
                        "<div class='pull-right'><button class='btn btn-primary' data-toggle='modal' data-backdrop='static' data-target='#modal' onclick='edit(" + variable.id + ")'>\
                    <i class='fa fa-pencil'></i>\
                    </button>\
                    <button class='btn btn-danger' onclick='deleteVariable(" + variable.id + ")'>\
                    <i class='fa fa-trash-o'></i>\
                    </button></div>"
                    ]).draw()
                })
            } else {
                $("#emptyMessage").show()
            }
        })
        .error(function () {
            errorFlash("Error fetching variables")
        })
}

$(document).ready(function () {
    load()
    // Setup the event listeners
    // Handle manual additions
    $("#conditionForm").submit(function () {
        // Validate the form data
        var conditionForm = document.getElementById("conditionForm")
        if (!conditionForm.checkValidity()) {
            conditionForm.reportValidity()
            return
        }
        addCondition(
            $("#condition").val(),
            $("#value").val());
        conditions.DataTable().draw();

        // Reset user input.
        $("#conditionForm>div>input").val('');
        $("#condition").focus();
        return false;
    });
    // Handle Deletion
    $("#conditionsTable").on("click", "span>i.fa-trash-o", function () {
        conditions.DataTable()
            .row($(this).parents('tr'))
            .remove()
            .draw();
    });
    $("#modal").on("hide.bs.modal", function () {
        dismiss();
    });
    $("#csv-template").click(downloadCSVTemplate)
});