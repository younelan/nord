
    // Define widget creation functions
        // Define widget creation functions
        function createStatusWidget(label, statusClass) {
            return `<div class="status-bubble status-${statusClass}">${label}</div>`;
            }
            
            function createTextWidget(label, value) {
            return `<div class="text-metric">${label}</div><div class="text-value">${value}</div>
            \n`;
            }
            
            function createPercentWidget(label, value) {
            return `<div class="text-metric">${label}</div><div class="percent-bar">\n<div class="percent-fill" style="width: ${value}%;"></div></div>\n`;
            }
            
            function createHistogramWidget(label, values) {
            const bars = values.map(value => `<div class="histogram-bar" style="height: ${value}%;"></div>`).join('');
            return `<div class="text-metric">${label}</div><div class="histogram">${bars}</div>\n`;
            }
            
    
    

    // Function to generate HTML for metrics section
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
        // Define the createRow function
        function createListRow(category, metrics) {
            let rowContent = `<div class="metrics-row"><div class="column-header">${category}</div>`;
            metrics.forEach(metric => {
                //rowContent = "<span >"

                switch (metric.type) {
                case 'status':
                    rowContent += createStatusWidget(metric.label, metric.class);
                    break;
                case 'text':
                    rowContent += createTextWidget(metric.label, metric.value);
                    break;
                case 'percent':
                    rowContent += createPercentWidget(metric.label, metric.value);
                    break;
                case 'histogram':
                    if(metric.value!==undefined) {
                        //alert("hello")
                        rowContent += createHistogramWidget(metric.label, metric.value);

                    } else {
                        console.log(metric)                        
                    }
                    break;
                }
                //rowContent += "</span>";
            });
            rowContent += '</div>';
            return rowContent;
        }

    // Function to create a row of metrics for device Details
    function createDeviceRow(metrics) {
        let rowContent = '';

        metrics.forEach(metric => {
            switch (metric.type) {
                case 'status':
                    rowContent += createStatusWidget(metric.label, metric.class);
                    break;
                case 'text':
                    rowContent += createTextWidget(metric.label, metric.value);
                    break;
                case 'percent':
                    rowContent += createPercentWidget(metric.label, metric.value);
                    break;
                case 'histogram':
                    rowContent += createHistogramWidget(metric.label, metric.value);
                    break;
            }
        });
        return rowContent;
    }

    // Function to render device details
    function renderDeviceDetails(device) {
        const deviceDetailsContainer = document.getElementById('deviceDetails');
        let htmlString = `
            <div class="device ${device.status ? getStatusClass(device.status) : ''}">
                <div class="device-header toggle-details collapsible ${device.status ? getStatusClass(device.status) : ''}">
                    <i class="fas fa-desktop"></i>
                    <h2>${device.name}</h2>
                    <div class="actions">
                        ${device.actions.map(action => `<button>${action.name}</button>`).join('')}
                    </div>
                    <div class="toggle-details-icon">-</div>
                    <i class="fas fa-chevron-down"></i>
                </div>
                <div class="device-content content active">
                    ${device.sections.map(section => `
                        <div class="section ${section.collapsed ? 'collapsed' : ''} ${section.status ? getStatusClass(section.status) : ''}">
                            ${section.title ? `
                                <div class="section-header collapsible ${section.status ? getStatusClass(section.status) : ''}">
                                    <i class="fas ${section.icon}"></i>
                                    <h3>${section.title}</h3>
                                    <i class="fas fa-chevron-down"></i>
                                </div>
                            ` : ''}
                            <div class="section-content ${section.collapsed ? '' : 'active'}">
                                ${renderSectionContent(section)}
                            </div>
                        </div>
                    `).join('')}
                </div>
            </div>
        `;
        deviceDetailsContainer.innerHTML = htmlString;

        // Add click event listener to toggle details button
        const toggleDetailsButton = document.querySelector('.toggle-details');
        const toggleDetailsIcon = document.querySelector('.toggle-details-icon');
        toggleDetailsButton.addEventListener('click', () => {
            const detailsContent = document.querySelectorAll('.section-content');
            detailsContent.forEach(content => {
                content.classList.toggle('active');
            });
            toggleDetailsIcon.innerText = toggleDetailsIcon.innerText === '-' ? '+' : '-';
        });

        // Add click event listeners to collapsible section headers
        const collapsibleHeaders = document.querySelectorAll('.section-header.collapsible');
        collapsibleHeaders.forEach(header => {
            header.addEventListener('click', () => {
                const content = header.nextElementSibling;
                content.classList.toggle('active');
            });
        });
    }

    // Function to render section content based on type
    function renderSectionContent(section) {
        let sectionContent = section.text !== undefined ? `<p>${section.text}</p>` : '';
        switch (section.type) {
            case 'table':
                sectionContent += section.data ? generateTableHTML(section.data) : '';
                break;
            case 'histogram':
                sectionContent += section.values !== undefined && section.values.length > 0 ? createHistogramWidget(section.title, section.values) : '';
                break;
            case 'metrics':
                sectionContent += section.metrics ? generateMetricsHTML(section.metrics) : '';
                break;
            default:
                break;
        }
        return sectionContent;
    }

    // Function to generate HTML for table data
    function generateTableHTML(data) {
        return `
            <table>
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

    // Function to get status class based on status value
    function getStatusClass(status) {
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


