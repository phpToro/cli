// Forward console.log/error/warn/info to native for dev server log streaming
(function() {
    var origLog = console.log, origError = console.error, origWarn = console.warn, origInfo = console.info;
    function forward(level, args) {
        try {
            var msg = Array.prototype.slice.call(args).map(function(a) {
                return typeof a === 'object' ? JSON.stringify(a) : String(a);
            }).join(' ');
            window.webkit.messageHandlers.phpToro.postMessage({type:'log', level:level, message:msg});
        } catch(e) {}
    }
    console.log = function() { forward('log', arguments); origLog.apply(console, arguments); };
    console.error = function() { forward('error', arguments); origError.apply(console, arguments); };
    console.warn = function() { forward('warn', arguments); origWarn.apply(console, arguments); };
    console.info = function() { forward('info', arguments); origInfo.apply(console, arguments); };
})();

window.phpToro = {
    action: function(id) {
        window.webkit.messageHandlers.phpToro.postMessage({type:'action',id:id});
    },
    bind: function(field, value) {
        window.webkit.messageHandlers.phpToro.postMessage({type:'bind',field:field,value:String(value)});
        document.querySelectorAll('[data-bind="'+field+'"]').forEach(function(el) {
            var fmt = el.getAttribute('data-bind-format');
            el.textContent = fmt ? fmt.replace('{}', value) : value;
        });
    },
    callback: function(ref, data) {
        window.webkit.messageHandlers.phpToro.postMessage({type:'callback',ref:ref,data:data||{}});
    },
    segment: function(btn, field, value) {
        var parent = btn.parentElement;
        parent.querySelectorAll('.pt-segment-btn').forEach(function(b) { b.classList.remove('pt-segment-active'); });
        btn.classList.add('pt-segment-active');
        phpToro.bind(field, value);
    },
    toggle: function(el, field) {
        phpToro.bind(field, String(el.checked));
    }
};

// Context menu support (macOS)
document.addEventListener('contextmenu', function(e) {
    var el = e.target.closest('[data-context-menu]');
    if (el) {
        e.preventDefault();
        var menuId = el.getAttribute('data-context-menu');
        var itemData = el.getAttribute('data-context-data');
        window.webkit.messageHandlers.phpToro.postMessage({
            type: 'contextMenu',
            menuId: menuId,
            data: itemData ? JSON.parse(itemData) : {},
            x: e.clientX,
            y: e.clientY
        });
    }
});

// Hide broken images
document.addEventListener('error', function(e) {
    if (e.target.tagName === 'IMG') { e.target.style.display = 'none'; }
}, true);

// Touch feedback for tappable elements
document.addEventListener('touchstart', function(e) {
    var el = e.target.closest('[onclick]');
    if (el) el.classList.add('phptoro-active');
}, {passive: true});
document.addEventListener('touchend', function(e) {
    var el = e.target.closest('.phptoro-active');
    if (el) setTimeout(function() { el.classList.remove('phptoro-active'); }, 100);
}, {passive: true});
document.addEventListener('touchcancel', function(e) {
    document.querySelectorAll('.phptoro-active').forEach(function(el) { el.classList.remove('phptoro-active'); });
}, {passive: true});
