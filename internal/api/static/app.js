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
        '</div>';

    modal.classList.remove('hidden');
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
