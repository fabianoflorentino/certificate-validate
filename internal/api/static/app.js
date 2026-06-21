const API_BASE = '/api/v1/cert';
let autoRefreshTimer = null;

const state = {
    certificates: [],
    errors: [],
    searchTerm: '',
    sortBy: 'daysLeft-asc'
};

async function fetchCertificates() {
    const loading = document.getElementById('loading');
    const error = document.getElementById('error');
    const grid = document.getElementById('cardsGrid');

    loading.classList.remove('hidden');
    error.classList.add('hidden');
    grid.innerHTML = '';

    try {
        const response = await fetch(API_BASE + '/info/all');
        if (!response.ok) {
            throw new Error('HTTP ' + response.status + ': ' + response.statusText);
        }

        const data = await response.json();
        state.certificates = data.certificates || [];
        state.errors = data.errors || [];
        render();

        const now = new Date();
        document.getElementById('lastUpdated').textContent =
            'Last updated: ' + now.toLocaleTimeString();
    } catch (err) {
        error.textContent = 'Failed to load certificates: ' + err.message;
        error.classList.remove('hidden');
    } finally {
        loading.classList.add('hidden');
    }
}

function render() {
    const grid = document.getElementById('cardsGrid');
    const empty = document.getElementById('emptyState');
    grid.innerHTML = '';
    empty.classList.add('hidden');

    // Filter by search term
    const term = state.searchTerm.toLowerCase();
    let filtered = state.certificates;
    if (term) {
        filtered = filtered.filter(function (cert) {
            return cert.hostname.toLowerCase().indexOf(term) !== -1 ||
                cert.commonName.toLowerCase().indexOf(term) !== -1 ||
                cert.issuer.toLowerCase().indexOf(term) !== -1 ||
                (cert.subjectAltName || []).some(function (san) {
                    return san.toLowerCase().indexOf(term) !== -1;
                });
        });
    }

    // Sort
    var sortBy = state.sortBy;
    filtered = filtered.slice().sort(function (a, b) {
        if (sortBy === 'daysLeft-asc') return (a.daysLeft || 0) - (b.daysLeft || 0);
        if (sortBy === 'daysLeft-desc') return (b.daysLeft || 0) - (a.daysLeft || 0);
        if (sortBy === 'hostname') return a.hostname.localeCompare(b.hostname);
        if (sortBy === 'issuer') return a.issuer.localeCompare(b.issuer);
        return 0;
    });

    // Update summary badges
    updateSummary(state.certificates);

    // Show fetch errors
    if (state.errors.length > 0) {
        var section = document.createElement('div');
        section.className = 'fetch-errors';
        var h = document.createElement('h3');
        h.textContent = 'Errors (' + state.errors.length + ')';
        section.appendChild(h);
        state.errors.forEach(function (msg) {
            var p = document.createElement('p');
            p.className = 'fetch-error-item';
            p.textContent = msg;
            section.appendChild(p);
        });
        grid.appendChild(section);
    }

    // Empty state
    if (filtered.length === 0) {
        if (term) {
            empty.textContent = 'No hosts match "' + state.searchTerm + '"';
        } else {
            empty.innerHTML = 'No certificates found. Add hosts to <code>config/settings.yml</code>.';
        }
        empty.classList.remove('hidden');
        return;
    }

    // Render cards
    filtered.forEach(function (cert) {
        grid.appendChild(createCard(cert));
    });
}

function handleSearch() {
    state.searchTerm = document.getElementById('searchInput').value;
    render();
}

function handleSort() {
    state.sortBy = document.getElementById('sortSelect').value;
    render();
}

function updateSummary(certs) {
    var container = document.getElementById('summaryBadges');
    var critical = 0;
    var warning = 0;
    var good = 0;
    for (var i = 0; i < certs.length; i++) {
        var d = certs[i].daysLeft;
        if (d <= 7) critical++;
        else if (d <= 30) warning++;
        else good++;
    }
    var parts = [];
    if (critical) parts.push('<span class="badge-summary badge-critical">\u25CF ' + critical + '</span>');
    if (warning) parts.push('<span class="badge-summary badge-warning">\u25CF ' + warning + '</span>');
    if (good) parts.push('<span class="badge-summary badge-good">\u25CF ' + good + '</span>');
    container.innerHTML = parts.join('') || '<span class="badge-summary">0 hosts</span>';
}

function createCard(cert) {
    const card = document.createElement('div');
    card.className = 'cert-card';
    card.onclick = function () { showModal(cert); };

    const daysLeft = cert.daysLeft;
    const statusClass = daysLeft > 30 ? 'status-good' : daysLeft > 7 ? 'status-warn' : 'status-critical';
    const typeBadge = cert.type.split(' ').slice(0, 2).join(' ');

    card.innerHTML =
        '<div class="card-header">' +
            '<div class="card-hostline">' +
                '<span class="card-hostname">' + esc(cert.hostname) + '</span>' +
                '<span class="card-port">:' + cert.port + '</span>' +
            '</div>' +
            '<span class="card-days-compact ' + statusClass + '">' +
                daysLeft + ' <span class="days-unit">d</span>' +
            '</span>' +
        '</div>' +
        '<div class="card-common-name">' + esc(cert.commonName) + '</div>' +
        '<div class="card-meta">' +
            '<span class="cert-type-badge">' + esc(typeBadge) + '</span>' +
            '<span class="card-issuer" title="' + escAttr(cert.issuer) + '">' + esc(cert.issuer) + '</span>' +
        '</div>';

    return card;
}

function showModal(cert) {
    const modal = document.getElementById('modal');
    const body = document.getElementById('modalBody');

    const daysLeft = cert.daysLeft;
    const statusClass = daysLeft > 30 ? 'status-good' : daysLeft > 7 ? 'status-warn' : 'status-critical';
    const statusLabel = daysLeft > 30 ? 'Healthy' : daysLeft > 7 ? 'Expiring Soon' : 'Critical';

    var sansHtml = '';
    if (cert.subjectAltName && cert.subjectAltName.length > 0) {
        var items = cert.subjectAltName.map(function (san) {
            return '<code>' + esc(san) + '</code>';
        });
        sansHtml = items.join(', ');
    } else {
        sansHtml = '<span class="text-muted">None</span>';
    }

    var crlHtml = '';
    if (cert.crl && cert.crl.length > 0) {
        var crlItems = cert.crl.map(function (url) {
            return '<code>' + esc(url) + '</code>';
        });
        crlHtml =
            '<div class="detail-row">' +
                '<span class="detail-label">CRL Distribution Points</span>' +
                '<span class="detail-value">' + crlItems.join('<br>') + '</span>' +
            '</div>';
    }

    var tlsHtml = '';
    if (cert.tlsVersion || cert.cipherSuite) {
        tlsHtml =
            '<div class="detail-section">' +
                '<span class="detail-section-title">Connection Security</span>' +
                '<div class="detail-row">' +
                    '<span class="detail-label">TLS Version</span>' +
                    '<span class="detail-value">' + esc(cert.tlsVersion || '—') + '</span>' +
                '</div>' +
                '<div class="detail-row">' +
                    '<span class="detail-label">Cipher Suite</span>' +
                    '<span class="detail-value"><code>' + esc(cert.cipherSuite || '—') + '</code></span>' +
                '</div>' +
            '</div>';
    }

    var chainHtml = '';
    if (cert.chain && cert.chain.length > 0) {
        var items = [];
        for (var i = cert.chain.length - 1; i >= 0; i--) {
            var entry = cert.chain[i];
            var label = i === 0 ? 'Leaf' : i === cert.chain.length - 1 ? 'Root' : 'Intermediate';
            var arrow = i < cert.chain.length - 1 ? '<div class="chain-arrow">\u25BC</div>' : '';
            items.push(
                arrow +
                '<div class="chain-entry">' +
                    '<div class="chain-entry-header">' +
                        '<span class="chain-label">' + label + '</span>' +
                        '<code class="chain-fingerprint">' + esc(entry.fingerprint.substring(0, 16)) + '&hellip;</code>' +
                    '</div>' +
                    '<div class="chain-meta">' +
                        '<span class="chain-subject">' + esc(entry.subject) + '</span>' +
                    '</div>' +
                    '<div class="chain-meta">' +
                        '<span class="chain-detail">Issued by: ' + esc(entry.issuer) + '</span>' +
                    '</div>' +
                    '<div class="chain-meta">' +
                        '<span class="chain-detail">Expires: ' + esc(entry.notAfter) + '</span>' +
                    '</div>' +
                '</div>'
            );
        }
        chainHtml =
            '<div class="detail-section">' +
                '<span class="detail-section-title">Certificate Chain</span>' +
                '<div class="chain-nav">' + items.join('') + '</div>' +
            '</div>';
    }

    body.innerHTML =
        '<div class="detail-grid">' +
            '<div class="detail-row">' +
                '<span class="detail-label">Hostname</span>' +
                '<span class="detail-value">' + esc(cert.hostname) + '</span>' +
            '</div>' +
            '<div class="detail-row">' +
                '<span class="detail-label">Port</span>' +
                '<span class="detail-value">' + cert.port + '</span>' +
            '</div>' +
            '<div class="detail-row">' +
                '<span class="detail-label">Common Name</span>' +
                '<span class="detail-value">' + esc(cert.commonName) + '</span>' +
            '</div>' +
            '<div class="detail-row">' +
                '<span class="detail-label">Issuer</span>' +
                '<span class="detail-value">' + esc(cert.issuer) + '</span>' +
            '</div>' +
            '<div class="detail-row">' +
                '<span class="detail-label">Certificate Type</span>' +
                '<span class="detail-value">' + esc(cert.type) + '</span>' +
            '</div>' +
            '<div class="detail-row">' +
                '<span class="detail-label">Valid From</span>' +
                '<span class="detail-value">' + esc(cert.notBefore) + '</span>' +
            '</div>' +
            '<div class="detail-row">' +
                '<span class="detail-label">Valid Until</span>' +
                '<span class="detail-value">' + esc(cert.notAfter) + '</span>' +
            '</div>' +
            '<div class="detail-row">' +
                '<span class="detail-label">Days Left</span>' +
                '<span class="detail-value">' +
                    '<span class="status-dot ' + statusClass + '"></span>' +
                    daysLeft + ' (' + statusLabel + ')' +
                '</span>' +
            '</div>' +
            '<div class="detail-row">' +
                '<span class="detail-label">Subject Alt Names</span>' +
                '<span class="detail-value">' + sansHtml + '</span>' +
            '</div>' +
            crlHtml +
            tlsHtml +
            chainHtml +
            '<div id="historyChart" class="detail-section">' +
                '<span class="detail-section-title">Days Left History</span>' +
                '<div class="chart-container"><div class="chart-loading">Loading history...</div></div>' +
            '</div>' +
        '</div>';

    modal.classList.remove('hidden');

    fetchHistoryChart(cert.hostname);
}

function fetchHistoryChart(hostname) {
    var container = document.querySelector('#historyChart .chart-container');
    if (!container) return;

    fetch(API_BASE + '/history/' + encodeURIComponent(hostname))
        .then(function (r) {
            if (!r.ok) throw new Error('HTTP ' + r.status);
            return r.json();
        })
        .then(function (entries) {
            if (!entries || entries.length === 0) {
                container.innerHTML = '<div class="chart-empty">No history data yet.</div>';
                return;
            }
            renderSVGChart(entries, container);
        })
        .catch(function (err) {
            container.innerHTML = '<div class="chart-empty">History unavailable: ' + err.message + '</div>';
        });
}

function renderSVGChart(entries, container) {
    var data = entries.slice(0, 30).reverse();

    if (data.length < 2) {
        container.innerHTML = '<div class="chart-empty">Need at least 2 data points.</div>';
        return;
    }

    var W = 500;
    var H = 160;
    var PAD = { top: 8, right: 8, bottom: 28, left: 36 };
    var innerW = W - PAD.left - PAD.right;
    var innerH = H - PAD.top - PAD.bottom;

    var minDays = Infinity;
    var maxDays = -Infinity;
    for (var i = 0; i < data.length; i++) {
        var d = data[i].daysLeft;
        if (d < minDays) minDays = d;
        if (d > maxDays) maxDays = d;
    }
    var range = maxDays - minDays || 1;
    var yPad = range * 0.1;
    var yMin = Math.max(0, minDays - yPad);
    var yMax = maxDays + yPad;

    function xPos(i) {
        return PAD.left + (i / (data.length - 1)) * innerW;
    }

    function yPos(v) {
        return PAD.top + (1 - (v - yMin) / (yMax - yMin)) * innerH;
    }

    var pts = [];
    for (var i = 0; i < data.length; i++) {
        pts.push(xPos(i).toFixed(1) + ',' + yPos(data[i].daysLeft).toFixed(1));
    }

    function fmtDate(ts) {
        var d = new Date(ts);
        return d.toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
    }

    var yTicks = [];
    var step = (yMax - yMin) / 3;
    for (var t = yMin; t <= yMax + 0.1; t += step) {
        yTicks.push(Math.round(t));
    }

    var xTickCount = Math.min(5, data.length);
    var xStep = Math.max(1, Math.floor((data.length - 1) / (xTickCount - 1)));
    var xTicks = [];
    for (var i = 0; i < data.length; i += xStep) {
        xTicks.push(i);
    }
    if (xTicks[xTicks.length - 1] !== data.length - 1) {
        xTicks.push(data.length - 1);
    }

    var tooltip = document.createElement('div');
    tooltip.className = 'chart-tooltip hidden';

    var svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
    svg.setAttribute('viewBox', '0 0 ' + W + ' ' + H);
    svg.setAttribute('class', 'history-svg');

    for (var t = 0; t < yTicks.length; t++) {
        var y = yPos(yTicks[t]);
        var line = document.createElementNS('http://www.w3.org/2000/svg', 'line');
        line.setAttribute('x1', PAD.left);
        line.setAttribute('y1', y);
        line.setAttribute('x2', PAD.left + innerW);
        line.setAttribute('y2', y);
        line.setAttribute('stroke', 'var(--border)');
        line.setAttribute('stroke-width', '0.5');
        svg.appendChild(line);

        var label = document.createElementNS('http://www.w3.org/2000/svg', 'text');
        label.setAttribute('x', PAD.left - 4);
        label.setAttribute('y', y + 3);
        label.setAttribute('text-anchor', 'end');
        label.setAttribute('fill', 'var(--text-muted)');
        label.setAttribute('font-size', '9');
        label.textContent = yTicks[t];
        svg.appendChild(label);
    }

    for (var t = 0; t < xTicks.length; t++) {
        var i = xTicks[t];
        var x = xPos(i);
        var label = document.createElementNS('http://www.w3.org/2000/svg', 'text');
        label.setAttribute('x', x);
        label.setAttribute('y', H - 4);
        label.setAttribute('text-anchor', 'middle');
        label.setAttribute('fill', 'var(--text-muted)');
        label.setAttribute('font-size', '8');
        label.textContent = fmtDate(data[i].timestamp);
        svg.appendChild(label);
    }

    var areaPts = pts.slice();
    areaPts.push((PAD.left + innerW).toFixed(1) + ',' + yPos(yMin).toFixed(1));
    areaPts.push(PAD.left.toFixed(1) + ',' + yPos(yMin).toFixed(1));
    var area = document.createElementNS('http://www.w3.org/2000/svg', 'polygon');
    area.setAttribute('points', areaPts.join(' '));
    area.setAttribute('fill', 'var(--primary)');
    area.setAttribute('opacity', '0.1');
    svg.appendChild(area);

    var polyline = document.createElementNS('http://www.w3.org/2000/svg', 'polyline');
    polyline.setAttribute('points', pts.join(' '));
    polyline.setAttribute('fill', 'none');
    polyline.setAttribute('stroke', 'var(--primary)');
    polyline.setAttribute('stroke-width', '1.5');
    polyline.setAttribute('stroke-linejoin', 'round');
    svg.appendChild(polyline);

    for (var i = 0; i < data.length; i++) {
        var cx = xPos(i);
        var cy = yPos(data[i].daysLeft);
        var circle = document.createElementNS('http://www.w3.org/2000/svg', 'circle');
        circle.setAttribute('cx', cx);
        circle.setAttribute('cy', cy);
        circle.setAttribute('r', '2.5');
        circle.setAttribute('fill', 'var(--primary)');
        svg.appendChild(circle);
    }

    var overlay = document.createElementNS('http://www.w3.org/2000/svg', 'rect');
    overlay.setAttribute('x', PAD.left);
    overlay.setAttribute('y', PAD.top);
    overlay.setAttribute('width', innerW);
    overlay.setAttribute('height', innerH);
    overlay.setAttribute('fill', 'transparent');
    overlay.addEventListener('mousemove', function (e) {
        var rect = svg.getBoundingClientRect();
        var chartX = (e.clientX - rect.left) / rect.width * W;
        var relX = chartX - PAD.left;
        var idx = Math.max(0, Math.min(data.length - 1, Math.round((relX / innerW) * (data.length - 1))));

        showTooltip(tooltip, data[idx], xPos(idx), yPos(data[idx].daysLeft), svg, container);
    });
    overlay.addEventListener('mouseleave', function () {
        tooltip.classList.add('hidden');
    });
    svg.appendChild(overlay);

    container.innerHTML = '';
    container.appendChild(svg);
    container.appendChild(tooltip);
}

function showTooltip(tooltip, entry, cx, cy, svg, container) {
    tooltip.innerHTML =
        '<div class="chart-tip-date">' + new Date(entry.timestamp).toLocaleString() + '</div>' +
        '<div class="chart-tip-days">' + entry.daysLeft + ' days left</div>';
    tooltip.classList.remove('hidden');

    var svgRect = svg.getBoundingClientRect();
    var contRect = container.getBoundingClientRect();
    var scale = svgRect.width / 500;
    var tx = (cx * scale) + svgRect.left - contRect.left;
    var ty = (cy * scale) + svgRect.top - contRect.top;

    tooltip.style.left = Math.min(tx - tooltip.offsetWidth / 2, contRect.width - tooltip.offsetWidth - 4) + 'px';
    tooltip.style.top = Math.max(ty - tooltip.offsetHeight - 10, 4) + 'px';
}

function closeModal(event) {
    if (event && event.target !== event.currentTarget) return;
    document.getElementById('modal').classList.add('hidden');
}

function getPreferredTheme() {
    var saved = localStorage.getItem('theme');
    if (saved === 'dark' || saved === 'light') return saved;
    return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
}

function applyTheme(theme) {
    var isDark = theme === 'dark';
    document.body.classList.toggle('dark-mode', isDark);
    document.getElementById('themeToggle').textContent = isDark ? 'Light' : 'Dark';
}

function toggleTheme() {
    var current = document.body.classList.contains('dark-mode') ? 'dark' : 'light';
    var next = current === 'dark' ? 'light' : 'dark';
    localStorage.setItem('theme', next);
    applyTheme(next);
}

var darkModeMedia = window.matchMedia('(prefers-color-scheme: dark)');
darkModeMedia.addEventListener('change', function () {
    if (!localStorage.getItem('theme')) {
        applyTheme(darkModeMedia.matches ? 'dark' : 'light');
    }
});

function toggleAutoRefresh() {
    const checkbox = document.getElementById('autoRefreshCheckbox');
    if (checkbox.checked) {
        autoRefreshTimer = setInterval(fetchCertificates, 60000);
    } else {
        if (autoRefreshTimer) {
            clearInterval(autoRefreshTimer);
            autoRefreshTimer = null;
        }
    }
}

function downloadURL(url, filename) {
    var a = document.createElement('a');
    a.href = url;
    a.download = filename;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
}

function exportJSON() {
    downloadURL(API_BASE + '/export/json', 'certificates.json');
}

function exportCSV() {
    downloadURL(API_BASE + '/export/csv', 'certificates.csv');
}

function formatDate(dateStr) {
    if (!dateStr) return '';
    return dateStr.split(' ')[0];
}

function esc(str) {
    var div = document.createElement('div');
    div.appendChild(document.createTextNode(str));
    return div.innerHTML;
}

function escAttr(str) {
    return str.replace(/"/g, '&quot;').replace(/'/g, '&#39;');
}

document.addEventListener('DOMContentLoaded', function () {
    applyTheme(getPreferredTheme());
    fetchCertificates();
});
