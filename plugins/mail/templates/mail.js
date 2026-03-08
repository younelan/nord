document.addEventListener('DOMContentLoaded', function() {
    // Define global variables
    let columnOrder = Object.keys(fields); // Initial column order
    let sortOrders = {}; // Object to track sort order for each column

    // Function to format date
    function formatDate(timestamp) {
        const date = new Date(timestamp * 1000);
        const day = date.getDate();
        const month = date.getMonth() + 1;
        return `${day}-${month}`;
    }

    // Function to render the table based on column order
    const renderTable = function(data) {

        const table = document.getElementById('mail-queue-table');
        table.innerHTML = '';

        // Render table header
        let headerRow = '<tr>';
        headerRow += '<th scope="col" class="mail-checkbox"><input type="checkbox" id="select-all"></th>';
        columnOrder.forEach(column => {
            const isVisible = fields[column].visible ? '' : 'hidden-column';
            const sortIcon = sortOrders[column] === 'asc' ? '<i class="fas fa-sort-up"></i>' : (sortOrders[column] === 'desc' ? '<i class="fas fa-sort-down"></i>' : '');
            headerRow += `<th scope="col" class="${column} ${isVisible} sortable" data-column="${column}" draggable="true">${fields[column].label} ${sortIcon}</th>`;
        });
        headerRow += '</tr>';
        table.insertAdjacentHTML('beforeend', headerRow);

        // Render table body
        data.forEach((mail, index) => {
            let rowClass = index % 2 === 0 ? 'even' : 'odd';
            let row = `<tr class="mail-row ${rowClass}">`;
            row += `<td><input type="checkbox" class="mail-checkbox"></td>`;
            columnOrder.forEach(column => {
                const isVisible = fields[column].visible ? '' : 'hidden-column';
                const fieldValue = column === 'recipients' ? mail[column].map(recipient => recipient.address).join(", ") : (column === 'arrival_time' ? formatDate(mail[column]) : mail[column]);
                row += `<td class="${column} ${isVisible}">${fieldValue}</td>`;
            });
            row += '</tr>';
            table.insertAdjacentHTML('beforeend', row);
        });
        
        const selectAllCheckbox = document.getElementById('select-all');
        console.log(selectAllCheckbox)
        if (selectAllCheckbox) {
            selectAllCheckbox.addEventListener('click', function() {
                const isChecked = this.checked;
                document.querySelectorAll('.mail-checkbox').forEach(checkbox => checkbox.checked = isChecked);
            });
        }

    };

// Function to render the column list
const renderColumnList = function() {
    const columnMenu = document.getElementById('columnMenu');
    columnMenu.innerHTML = ''; // Clear existing items

    for (const field in fields) {
        if (fields.hasOwnProperty(field)) {
            const listItem = document.createElement('li');
            const anchor = document.createElement('a');
            anchor.href = '#';
            anchor.classList.add('dropdown-item', 'toggle-column');
            anchor.dataset.column = field;
            anchor.textContent = fields[field].label;
            listItem.appendChild(anchor);
            columnMenu.appendChild(listItem);
        }
    }
};

// Function to toggle column visibility
const toggleColumnVisibility = function(column) {
    const isVisible = fields[column].visible;
    const elements = document.querySelectorAll(`.${column}`);
    elements.forEach(element => {
        if (isVisible) {
            element.classList.add('hidden-column');
        } else {
            element.classList.remove('hidden-column');
        }
    });
    fields[column].visible = !isVisible; // Update visibility state
};
// Event listener for toggling column visibility
document.getElementById('columnMenu').addEventListener('click', function(event) {
    const column = event.target.dataset.column;
    if (column) {
        toggleColumnVisibility(column);
        renderTable(mailQueue); // Re-render table with updated visibility
        document.getElementById('columnMenu').style.display = 'none'; // Hide the overlay
    }
    if (event.target.classList.contains('toggle-column')) {
        const column = event.target.dataset.column;
        toggleColumnVisibility(column);
        renderTable(mailQueue); // Re-render table with updated visibility
        toggleOverlay(); // Hide the overlay
        document.getElementById('columnMenu').style.display = 'none'; // Hide the overlay
    }
});

// Function to update the overlay display
const toggleOverlay = function() {
    const columnMenu = document.getElementById('columnMenu');
    if (columnMenu.style.display === 'none') {
        columnMenu.style.display = 'block';
    } else {
        columnMenu.style.display = 'none';
    }
};
// Event listener for toggling the overlay
document.getElementById('columnToggle').addEventListener('click', toggleOverlay);

// Initial rendering
renderColumnList();
renderTable(mailQueue);
    // Function to update column order based on drag and drop
    const updateColumnOrder = function(newColumnOrder) {
        columnOrder = newColumnOrder;
        saveColumnSettings();
    };

    // Function to save column visibility and order to cookie
    const saveColumnSettings = function() {
        const columnVisibility = {};
        columnOrder.forEach(column => {
            columnVisibility[column] = fields[column].visible;
        });
        setCookie('columnVisibility', JSON.stringify(columnVisibility));
        setCookie('columnOrder', JSON.stringify(columnOrder));
    };

    // Function to load column visibility and order from cookie
    const loadColumnSettings = function() {
        const columnVisibilityCookie = getCookie('columnVisibility');
        const columnOrderCookie = getCookie('columnOrder');

        if (columnVisibilityCookie) {
            const columnVisibility = JSON.parse(columnVisibilityCookie);
            for (const column in columnVisibility) {
                fields[column].visible = columnVisibility[column];
            }
        }

        if (columnOrderCookie) {
            columnOrder = JSON.parse(columnOrderCookie);
        }
    };

    // Function to set a cookie
    const setCookie = function(name, value, days) {
        let expires = '';
        if (days) {
            const date = new Date();
            date.setTime(date.getTime() + (days * 24 * 60 * 60 * 1000));
            expires = '; expires=' + date.toUTCString();
        }
        document.cookie = name + '=' + (value || '') + expires + '; path=/';
    };

    // Function to get a cookie
    const getCookie = function(name) {
        const nameEQ = name + '=';
        const ca = document.cookie.split(';');
        for (let i = 0; i < ca.length; i++) {
            let c = ca[i];
            while (c.charAt(0) === ' ') c = c.substring(1, c.length);
            if (c.indexOf(nameEQ) === 0) return c.substring(nameEQ.length, c.length);
        }
        return null;
    };

    // Event listeners for column reordering using drag and drop
    let draggingColumn = null;

    document.addEventListener('dragstart', function(event) {
        if (event.target.getAttribute('draggable') === 'true') {
            draggingColumn = event.target.dataset.column;
        }
    });

    document.addEventListener('dragover', function(event) {
        event.preventDefault();
    });

    document.addEventListener('drop', function(event) {
        if (event.target.getAttribute('draggable') === 'true') {
            event.preventDefault();
            const droppedColumn = event.target.dataset.column;
            const newColumnOrder = [...columnOrder];
            const draggingIndex = newColumnOrder.indexOf(draggingColumn);
            const droppedIndex = newColumnOrder.indexOf(droppedColumn);
            newColumnOrder.splice(draggingIndex, 1); // Remove dragging column
            newColumnOrder.splice(droppedIndex, 0, draggingColumn); // Insert dragging column at dropped position
            updateColumnOrder(newColumnOrder);
            renderTable(mailQueue); // Re-render table with updated column order
        }
    });

    // Event listener for column sorting
    document.addEventListener('click', function(event) {
        if (event.target.classList.contains('sortable')) {
            const column = event.target.dataset.column;
            sortOrders[column] = sortOrders[column] === 'asc' ? 'desc' : 'asc'; // Toggle sort order

            // Reset sort orders for other columns
            for (const key in sortOrders) {
                if (key !== column) {
                    sortOrders[key] = undefined;
                }
            }

            // Sort the data based on the column and sort order
            const sortedData = mailQueue.sort((a, b) => {
                let result;
                if (column === 'recipients') {
                    const recipientsA = a[column].map(recipient => recipient.address).join('');
                    const recipientsB = b[column].map(recipient => recipient.address).join('');
                    result = recipientsA.localeCompare(recipientsB);
                } else if (typeof a[column] === 'string') {
                    result = a[column].localeCompare(b[column]);
                } else {
                    result = a[column] - b[column];
                }
                return sortOrders[column] === 'asc' ? result : -result; // Adjust sorting direction
            });

            renderTable(sortedData); // Render the sorted table

            // Update sort icon
            document.querySelectorAll('.sortable i').forEach(icon => icon.classList.remove('fas', 'fa-sort-up', 'fas', 'fa-sort-down'));
            const sortIconClass = sortOrders[column] === 'asc' ? 'fas fa-sort-up' : 'fas fa-sort-down';
            event.target.querySelector('i').classList.add(...sortIconClass.split(' '));
        }
    });

    // Event listener for column visibility toggling
    document.addEventListener('click', function(event) {
        if (event.target.classList.contains('toggle-column')) {
            event.preventDefault();
            const column = event.target.dataset.column;
            const isVisible = fields[column].visible;
            if (isVisible) {
                document.querySelectorAll(`.${column}`).forEach(element => element.classList.add('hidden-column'));
            } else {
                document.querySelectorAll(`.${column}`).forEach(element => element.classList.remove('hidden-column'));
            }
            fields[column].visible = !isVisible; // Update visibility state
            saveColumnSettings(); // Save column visibility state
        }
    });


    // Event listener for toggle select all button
    const toggleSelectAllButton = document.getElementById('toggle-select-all');
    if (toggleSelectAllButton) {
        toggleSelectAllButton.addEventListener('click', function() {
            const allChecked = document.querySelectorAll('.mail-checkbox:checked').length === document.querySelectorAll('.mail-checkbox').length;
            document.querySelectorAll('.mail-checkbox').forEach(checkbox => checkbox.checked = !allChecked);
            selectAllCheckbox.checked = allChecked;
        });
    }

    // Load column settings and render the table
    loadColumnSettings();
    renderTable(mailQueue);
});
