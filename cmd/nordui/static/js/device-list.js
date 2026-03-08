// Device list functionality

function populateHosts(hostsData) {
    const hostsContainer = document.getElementById('hostContainer');
    if (!hostsContainer) return;

    hostsContainer.innerHTML = '';
    const groups = {};

    hostsData.forEach(host => {
        const metricsHTML = populateMetricsContainer(host);
        const hostRow = createHostRow(host, metricsHTML);

        if (host.group) {
            if (!groups[host.group]) {
                groups[host.group] = {
                    header: createGroupHeader(host.group),
                    hosts: []
                };
            }
            groups[host.group].hosts.push(hostRow);
        } else {
            hostsContainer.insertAdjacentHTML('beforeend', hostRow);
        }
    });

    // Append groups
    for (let groupName in groups) {
        const groupHTML = `
            <div class="group" id="group-${groupName}">
                ${groups[groupName].header}
                <div class="group-details">
                    ${groups[groupName].hosts.join('')}
                </div>
            </div>
        `;
        hostsContainer.insertAdjacentHTML('beforeend', groupHTML);
    }
}

function createHostRow(host, metricsHTML) {
    const actionsHTML = host.actions ? host.actions.map(action => 
        `<a class="action" href="/device/${host.id}?page=${action.page}">
            <i class="fas fa-info-circle"></i>${action.name}
        </a>`
    ).join('') : '';

    return `
        <div class="host-row ${host.collapsed ? '' : 'expanded'}">
            <div class="hostname-header ${getStatusClass(host.status)}" onclick="toggleDetails(this)">
                <div class="hostname-icon">
                    <i class="fas fa-server"></i>
                </div>
                <div class="hostname-label">
                    ${host.name}
                    <div class="device-chevron">
                        <i class="fas fa-chevron-down"></i>
                    </div>
                </div>
                ${host.extendedStatus ? `<div class="host-summary">${host.extendedStatus}</div>` : ''}
            </div>
            <div class="details ${host.collapsed ? '' : 'show'}">
                <div class="action-buttons">
                    ${actionsHTML}
                </div>
                ${metricsHTML}
            </div>
        </div>
    `;
}

function createGroupHeader(groupName) {
    return `
        <div class="group-header" onclick="toggleGroupDetails('${groupName}')">
            <i class="fa fa-folder"></i> ${groupName}
            <div class="device-chevron">
                <i class="fas fa-chevron-down"></i>
            </div>
        </div>
    `;
}

function populateMetricsContainer(hostData) {
    let htmlContent = '';
    
    if (!hostData.metrics) return htmlContent;

    Object.keys(hostData.metrics).forEach(category => {
        const metrics = Object.values(hostData.metrics[category]);
        const categoryLabel = category.charAt(0).toUpperCase() + category.slice(1);
        htmlContent += createMetricsRow(categoryLabel, metrics);
    });
    
    return htmlContent;
}

function createMetricsRow(category, metrics) {
    let rowContent = `<div class="metrics-row"><div class="column-header">${category}</div>`;
    
    metrics.forEach(metric => {
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
    
    rowContent += '</div>';
    return rowContent;
}

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

function toggleDetails(element) {
    const details = element.nextElementSibling;
    const hostRow = element.closest('.host-row');
    details.classList.toggle('show');
    hostRow.classList.toggle('expanded');
}

function toggleGroupDetails(groupName) {
    const groupDetails = document.querySelector(`#group-${groupName} .group-details`);
    if (groupDetails) {
        groupDetails.classList.toggle('show');
    }
}

function toggleHosts(groupName) {
    const groupDetails = document.querySelector(`#group-${groupName} .group-details`);
    if (groupDetails) {
        const hostRows = groupDetails.querySelectorAll('.host-row .details');
        hostRows.forEach(details => {
            details.classList.toggle('show');
        });
    }
}
