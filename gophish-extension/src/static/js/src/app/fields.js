var fields = []

// Save attempts to POST or PUT to /fields/
function save(id) {
    var values = []
    $.each($("#valuesTable").DataTable().rows().data(), function (i, value) {
        values.push({
            email: unescapeHtml(value[0]),
            value: unescapeHtml(value[1])
        })
    })
    var field = {
        name: $("#name").val(),
        values: values
    }
    // Submit the field
    if (id != -1) {
        // If we're just editing an existing field,
        // we need to PUT /fields/:id
        field.id = id
        api.fieldId.put(field)
            .success(function (data) {
                successFlash("Field updated successfully!")
                load()
                dismiss()
                $("#modal").modal('hide')
            })
            .error(function (data) {
                modalError(data.responseJSON.message)
            })
    } else {
        // Else, if this is a new field, POST it
        // to /fields
        api.fields.post(field)
            .success(function (data) {
                successFlash("Field added successfully!")
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
    $("#valuesTable").dataTable().DataTable().clear().draw()
    $("#name").val("")
    $("#modal\\.flashes").empty()
}

function edit(id) {
    values = $("#valuesTable").dataTable({
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
        var field = {}
    } else {
        api.fieldId.get(id)
            .success(function (field) {
                $("#name").val(field.name)
                $.each(field.values, function (i, record) {
                    values.DataTable()
                        .row.add([
                            escapeHtml(record.email),
                            escapeHtml(record.value),
                            '<span style="cursor:pointer;"><i class="fa fa-trash-o"></i></span>'
                        ]).draw()
                });

            })
            .error(function () {
                errorFlash("Error fetching field")
            })
    }
    // Handle file uploads
    $("#csvvaluesupload").fileupload({
        url: "/api/import/field",
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
                addValue(
                    record.email,
                    record.value);
            });
            values.DataTable().draw();
        }
    })
}

var downloadCSVTemplate = function () {
    var csvScope = [{
        'Email': 'foobar@example.com',
        'Value': 'Example'
    }]
    var filename = 'field_template.csv'
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

var deleteField = function (id) {
    var field = fields.find(function (x) {
        return x.id === id
    })
    if (!field) {
        return
    }
    Swal.fire({
        title: "Are you sure?",
        text: "This will delete the field. This can't be undone!",
        type: "warning",
        animation: false,
        showCancelButton: true,
        confirmButtonText: "Delete " + escapeHtml(field.name),
        confirmButtonColor: "#428bca",
        reverseButtons: true,
        allowOutsideClick: false,
        preConfirm: function () {
            return new Promise(function (resolve, reject) {
                api.fieldId.delete(id)
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
                'Field Deleted!',
                'This field has been deleted!',
                'success'
            );
        }
        $('button:contains("OK")').on('click', function () {
            location.reload()
        })
    })
}

function addValue(emailInput, valueInput) {
    // Create new data row.
    var email = escapeHtml(emailInput).toLowerCase();
    var newRow = [
        email,
        escapeHtml(valueInput),
        '<span style="cursor:pointer;"><i class="fa fa-trash-o"></i></span>'
    ];

    // Check table to see if email already exists.
    var valuesTable = values.DataTable();
    var existingRowIndex = valuesTable
        .column(0, {
            order: "index"
        }) // Email column has index of 0
        .data()
        .indexOf(email);
    // Update or add new row as necessary.
    if (existingRowIndex >= 0) {
        valuesTable
            .row(existingRowIndex, {
                order: "index"
            })
            .data(newRow);
    } else {
        valuesTable.row.add(newRow);
    }
}

function load() {
    $("#fieldTable").hide()
    $("#emptyMessage").hide()
    $("#loading").show()
    api.fields.summary()
        .success(function (response) {
            $("#loading").hide()
            if (response.total > 0) {
                fields = response.fields
                $("#emptyMessage").hide()
                $("#fieldTable").show()
                var fieldTable = $("#fieldTable").DataTable({
                    destroy: true,
                    columnDefs: [{
                        orderable: false,
                        targets: "no-sort"
                    }]
                });
                fieldTable.clear();
                $.each(fields, function (i, field) {
                    fieldTable.row.add([
                        escapeHtml(field.name),
                        escapeHtml(field.num_values),
                        moment(field.modified_date).format('MMMM Do YYYY, h:mm:ss a'),
                        "<div class='pull-right'><button class='btn btn-primary' data-toggle='modal' data-backdrop='static' data-target='#modal' onclick='edit(" + field.id + ")'>\
                    <i class='fa fa-pencil'></i>\
                    </button>\
                    <button class='btn btn-danger' onclick='deleteField(" + field.id + ")'>\
                    <i class='fa fa-trash-o'></i>\
                    </button></div>"
                    ]).draw()
                })
            } else {
                $("#emptyMessage").show()
            }
        })
        .error(function () {
            errorFlash("Error fetching fields")
        })
}

$(document).ready(function () {
    load()
    // Setup the event listeners
    // Handle manual additions
    $("#valueForm").submit(function () {
        // Validate the form data
        var valueForm = document.getElementById("valueForm")
        if (!valueForm.checkValidity()) {
            valueForm.reportValidity()
            return
        }
        addValue(
            $("#email").val(),
            $("#value").val());
        values.DataTable().draw();

        // Reset user input.
        $("#valueForm>div>input").val('');
        $("#email").focus();
        return false;
    });
    // Handle Deletion
    $("#valuesTable").on("click", "span>i.fa-trash-o", function () {
        values.DataTable()
            .row($(this).parents('tr'))
            .remove()
            .draw();
    });
    $("#modal").on("hide.bs.modal", function () {
        dismiss();
    });
    $("#csv-template").click(downloadCSVTemplate)
});