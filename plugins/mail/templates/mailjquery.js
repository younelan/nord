$(document).ready(function() {
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
        const table = $('#mail-queue-table');
        table.empty();

        // Render table header
        let headerRow = '<tr>';
        headerRow += '<th scope="col" class="mail-checkbox"><input type="checkbox" id="select-all"></th>';
        columnOrder.forEach(column => {
            const isVisible = fields[column].visible ? '' : 'hidden-column';
            const sortIcon = sortOrders[column] === 'asc' ? '<i class="fas fa-sort-up"></i>' : (sortOrders[column] === 'desc' ? '<i class="fas fa-sort-down"></i>' : '');
            headerRow += `<th scope="col" class="${column} ${isVisible} sortable" data-column="${column}" draggable="true">${fields[column].label} ${sortIcon}</th>`;
        });
        headerRow += '</tr>';
        table.append(headerRow);

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
            table.append(row);
        });
    };

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

    $(document).on('dragstart', '[draggable="true"]', function(event) {
        draggingColumn = $(this).data('column');
    });

    $(document).on('dragover', '[draggable="true"]', function(event) {
        event.preventDefault();
    });

    $(document).on('drop', '[draggable="true"]', function(event) {
        event.preventDefault();
        const droppedColumn = $(this).data('column');
        const newColumnOrder = [...columnOrder];
        const draggingIndex = newColumnOrder.indexOf(draggingColumn);
        const droppedIndex = newColumnOrder.indexOf(droppedColumn);
        newColumnOrder.splice(draggingIndex, 1); // Remove dragging column
        newColumnOrder.splice(droppedIndex, 0, draggingColumn); // Insert dragging column at dropped position
        updateColumnOrder(newColumnOrder);
        renderTable(mailQueue); // Re-render table with updated column order
    });

    // Event listener for column sorting
    $(document).on('click', '.sortable', function() {
        const column = $(this).data('column');
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
        $('.sortable i').removeClass('fas fa-sort-up fas fa-sort-down');
        const sortIconClass = sortOrders[column] === 'asc' ? 'fas fa-sort-up' : 'fas fa-sort-down';
        $(this).find('i').addClass(sortIconClass);
    });

    const columnMenu = $('#columnMenu');
    for (const field in fields) {
        if (fields.hasOwnProperty(field)) {
            columnMenu.append(`<li><a class="dropdown-item toggle-column" href="#" data-column="${field}">${fields[field].label}</a></li>`);
        }
    }
    loadColumnSettings();

    renderTable(mailQueue);

    $('.toggle-column').click(function(e) {
        e.preventDefault();
        const column = $(this).data('column');
        const isVisible = fields[column].visible;
        if (isVisible) {
            $(`.${column}`).addClass('hidden-column');
        } else {
            $(`.${column}`).removeClass('hidden-column');
        }
        fields[column].visible = !isVisible; // Update visibility state
        saveColumnSettings(); // Save column visibility state
    });
        
    $('#select-all').click(function() {
        const isChecked = $(this).prop('checked');
        $('.mail-checkbox').prop('checked', isChecked);
    });

    // Event listener for the "Toggle Select All" button
    $('#toggle-select-all').click(function() {
        const allChecked = $('.mail-checkbox:checked').length === $('.mail-checkbox').length;
        $('.mail-checkbox').prop('checked', !allChecked);
        $('#select-all').prop('checked', allChecked);
    });
});

