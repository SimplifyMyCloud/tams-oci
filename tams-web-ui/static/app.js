// Auto-refresh migration status
if (window.location.pathname === '/migrations') {
    setInterval(() => {
        fetch('/migrations')
            .then(response => {
                if (response.ok) {
                    const runningJobs = document.querySelectorAll('[data-status="running"]');
                    if (runningJobs.length > 0) {
                        window.location.reload();
                    }
                }
            });
    }, 5000);
}

// API Status Check
function checkAPIStatus() {
    fetch('/health')
        .then(response => {
            const statusEl = document.getElementById('api-status');
            const statusDot = statusEl.nextElementSibling;
            if (response.ok) {
                statusEl.textContent = 'Connected';
                statusDot.className = 'w-2 h-2 bg-green-400 rounded-full animate-pulse';
            } else {
                statusEl.textContent = 'Disconnected';
                statusDot.className = 'w-2 h-2 bg-red-400 rounded-full';
            }
        })
        .catch(() => {
            const statusEl = document.getElementById('api-status');
            const statusDot = statusEl.nextElementSibling;
            statusEl.textContent = 'Error';
            statusDot.className = 'w-2 h-2 bg-red-400 rounded-full';
        });
}

// Check API status every 10 seconds
setInterval(checkAPIStatus, 10000);

// Asset functions
function startMigration(assetId) {
    if (confirm('Start migration for this asset?')) {
        const form = document.createElement('form');
        form.method = 'POST';
        form.action = '/migrations/new';
        
        const assetField = document.createElement('input');
        assetField.type = 'hidden';
        assetField.name = 'asset_id';
        assetField.value = assetId;
        
        const typeField = document.createElement('input');
        typeField.type = 'hidden';
        typeField.name = 'source_type';
        typeField.value = 'tape';
        
        const pathField = document.createElement('input');
        pathField.type = 'hidden';
        pathField.name = 'source_path';
        pathField.value = 'LTO-AUTO-' + assetId;
        
        form.appendChild(assetField);
        form.appendChild(typeField);
        form.appendChild(pathField);
        document.body.appendChild(form);
        form.submit();
    }
}

function archiveAsset(assetId) {
    if (confirm('Archive this asset? It will be moved to cold storage.')) {
        fetch(`/api/v1/assets/${assetId}/archive`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' }
        })
        .then(response => {
            if (response.ok) {
                alert('Asset archived successfully');
                window.location.reload();
            } else {
                alert('Failed to archive asset');
            }
        });
    }
}

function deleteAsset(assetId) {
    if (confirm('Are you sure you want to delete this asset? This action cannot be undone.')) {
        fetch(`/api/v1/assets/${assetId}`, {
            method: 'DELETE',
            headers: { 'Content-Type': 'application/json' }
        })
        .then(response => {
            if (response.ok) {
                alert('Asset deleted successfully');
                window.location.href = '/assets';
            } else {
                alert('Failed to delete asset');
            }
        });
    }
}

function downloadChunk(url) {
    window.open(url, '_blank');
}

// Migration functions
function cancelJob(jobId) {
    if (confirm('Cancel this migration job?')) {
        fetch(`/api/v1/migrations/${jobId}/cancel`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' }
        })
        .then(response => {
            if (response.ok) {
                window.location.reload();
            } else {
                alert('Failed to cancel job');
            }
        });
    }
}

function retryJob(jobId) {
    if (confirm('Retry this migration job?')) {
        fetch(`/api/v1/migrations/${jobId}/retry`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' }
        })
        .then(response => {
            if (response.ok) {
                window.location.reload();
            } else {
                alert('Failed to retry job');
            }
        });
    }
}

function viewJobDetails(jobId) {
    fetch(`/api/v1/migrations/${jobId}`)
        .then(response => response.json())
        .then(data => {
            alert(JSON.stringify(data, null, 2));
        });
}

// Progress bar animation
document.addEventListener('DOMContentLoaded', function() {
    const progressBars = document.querySelectorAll('.progress-bar');
    progressBars.forEach(bar => {
        const width = bar.style.width;
        bar.style.width = '0%';
        setTimeout(() => {
            bar.style.width = width;
        }, 100);
    });

    // Handle URL parameters for success/error messages
    const urlParams = new URLSearchParams(window.location.search);
    const success = urlParams.get('success');
    const error = urlParams.get('error');
    
    if (success) {
        showNotification(success, 'success');
        // Clean URL
        window.history.replaceState({}, document.title, window.location.pathname);
    }
    
    if (error) {
        showNotification(error, 'error');
        window.history.replaceState({}, document.title, window.location.pathname);
    }
});

function showNotification(message, type) {
    const div = document.createElement('div');
    div.className = `fixed top-4 right-4 px-4 py-3 rounded shadow-lg fade-in ${
        type === 'success' ? 'bg-green-500 text-white' : 'bg-red-500 text-white'
    }`;
    div.textContent = message;
    document.body.appendChild(div);
    
    setTimeout(() => {
        div.remove();
    }, 5000);
}

// Keyboard shortcuts
document.addEventListener('keydown', function(e) {
    // Alt+A: Go to Assets
    if (e.altKey && e.key === 'a') {
        window.location.href = '/assets';
    }
    // Alt+M: Go to Migrations
    if (e.altKey && e.key === 'm') {
        window.location.href = '/migrations';
    }
    // Alt+D: Go to Dashboard
    if (e.altKey && e.key === 'd') {
        window.location.href = '/';
    }
    // Alt+N: New Asset (if on assets page)
    if (e.altKey && e.key === 'n' && window.location.pathname === '/assets') {
        window.location.href = '/assets/new';
    }
});