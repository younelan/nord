// Device details functionality

function renderDeviceDetails(device) {
    const deviceDetailsContainer = document.getElementById('deviceDetails');
    if (!deviceDetailsContainer) return;

    const htmlString = `
        <div class="device ${getStatusClass(device.status)}">
            <div class="device-header">
                <div style="display: flex; align-items: center; gap: 15px;">
                    <i class="fas fa-desktop"></i>
                    <h2>${device.name}</h2>
                </div>
                <div class="actions">
                    ${device.actions ? device.actions.map(action => 
                        `<button class="btn btn-secondary">${action.name}</button>`
                    ).join('') : ''}
                </div>
            </div>
            <div class="device-content">
                ${device.sections ? device.sections.map(section => renderSection(section)).join('') : ''}
                ${!device.sections ? renderMetricsSection(device.metrics) : ''}
            </div>
        </div>
    `;
    
    deviceDetailsContainer.innerHTML = htmlString;

    // Add event listeners for collapsible sections
    const collapsibleHeaders = document.querySelectorAll('.section-header');
    collapsibleHeaders.forEach(header => {
        header.addEventListener('click', () => {
            const content = header.nextElementSibling;
            content.classList.toggle('active');
            header.querySelector('.fa-chevron-down').classList.toggle('rotate');
        });
    });
}

function renderSection(section) {
    return `
        <div class="section ${section.collapsed ? 'collapsed' : ''} ${getStatusClass(section.status)}">
            ${section.title ? `
                <div class="section-header ${getStatusClass(section.status)}">
                    <i class="fas ${section.icon || 'fa-info-circle'}"></i>
                    <h3>${section.title}</h3>
                    <i class="fas fa-chevron-down"></i>
                </div>
            ` : ''}
            <div class="section-content ${section.collapsed ? '' : 'active'}">
                ${renderSectionContent(section)}
            </div>
        </div>
    `;
}

function renderSectionContent(section) {
    let content = section.text !== undefined ? `<p>${section.text}</p>` : '';
    
    switch (section.type) {
        case 'table':
            content += section.data ? generateTableHTML(section.data) : '';
            break;
        case 'histogram':
            content += section.values && section.values.length > 0 ? 
                createHistogramWidget(section.title, section.values) : '';
            break;
        case 'metrics':
            content += section.metrics ? generateMetricsHTML(section.metrics) : '';
            break;
        case 'text':
            // Already handled above
            break;
        default:
            break;
    }
    
    return content;
}

function renderMetricsSection(metrics) {
    if (!metrics) return '';
    return `
        <div class="section">
            <div class="section-header">
                <i class="fas fa-chart-line"></i>
                <h3>Metrics</h3>
            </div>
            <div class="section-content active">
                ${generateMetricsHTML(metrics)}
            </div>
        </div>
    `;
}

function generateMetricsHTML(metrics) {
    let metricsHTML = '';
    Object.keys(metrics).forEach(category => {
        metricsHTML += `<div class='column-row'>`;
        metricsHTML += `<div class="column-header">${category}</div>`;
        metricsHTML += createDeviceRow(metrics[category]);
        metricsHTML += `</div>`;
    });
    return metricsHTML;
}

function createDeviceRow(metrics) {
    let rowContent = '';
    
    Object.values(metrics).forEach(metric => {
        switch (metric.type) {
            case 'status':
                rowContent += createStatusWidget(metric.label, metric.class || metric.value);
                break;
            case 'text':
                rowContent += createTextWidget(metric.label, metric.value);
                break;
            case 'percent':
                rowContent += createPercentWidget(metric.label, metric.value);
                break;
            case 'histogram':
                if (metric.value !== undefined) {
                    rowContent += createHistogramWidget(metric.label, metric.value);
                }
                break;
        }
    });
    
    return rowContent;
}

function generateTableHTML(data) {
    if (!Array.isArray(data) || data.length === 0) return '';
    
    return `
        <table class="table">
            ${data.map((row, rowIndex) => `
                <tr>
                    ${row.map((cell, cellIndex) => `
                        <${rowIndex === 0 || cellIndex === 0 ? 'th' : 'td'}>${cell}</${rowIndex === 0 || cellIndex === 0 ? 'th' : 'td'}>
                    `).join('')}
                </tr>
            `).join('')}
        </table>
    `;
}

// Widget creation functions (reused from device-list.js)
function createStatusWidget(label, statusClass) {
    return `<div class="status-bubble status-${statusClass}">${label}</div>`;
}

function createTextWidget(label, value) {
    return `<div class="text-metric">${label}: <span class="text-value">${value}</span></div>`;
}

function createPercentWidget(label, value) {
    const percentValue = parseFloat(value) || 0;
    return `
        <div class="text-metric">
            ${label}
            <div class="percent-bar">
                <div class="percent-fill" style="width: ${percentValue}%;"></div>
            </div>
            <span class="text-value">${percentValue}%</span>
        </div>
    `;
}

function createHistogramWidget(label, values) {
    if (!Array.isArray(values)) return '';
    const bars = values.map(value => 
        `<div class="histogram-bar" style="height: ${value}%;"></div>`
    ).join('');
    return `
        <div class="text-metric">
            ${label}
            <div class="histogram">${bars}</div>
        </div>
    `;
}
