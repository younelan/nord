// Common utility functions

function toggleMenu() {
    const menu = document.getElementById('navbarMenu');
    menu.classList.toggle('show');
}

function toggleCollapseAll() {
    const detailsElements = document.querySelectorAll('.details');
    const allExpanded = Array.from(detailsElements).every(el => el.classList.contains('show'));
    
    detailsElements.forEach(details => {
        if (allExpanded) {
            details.classList.remove('show');
            details.closest('.host-row').classList.remove('expanded');
        } else {
            details.classList.add('show');
            details.closest('.host-row').classList.add('expanded');
        }
    });
}

function refreshHosts() {
    location.reload();
}

// Format values
function formatValue(value, type) {
    if (type === 'percent') {
        return parseFloat(value) || 0;
    }
    return value;
}

// Get status class
function getStatusClass(status) {
    if (!status) return '';
    switch (status.toLowerCase()) {
        case 'up':
            return 'up';
        case 'down':
            return 'down';
        case 'warning':
            return 'warning';
        default:
            return '';
    }
}
