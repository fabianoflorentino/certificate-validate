(() => {
    const copyText = async (value) => {
        if (!navigator.clipboard) {
            return false;
        }
        try {
            await navigator.clipboard.writeText(value);
            return true;
        } catch {
            return false;
        }
    };

    document.querySelectorAll('[data-copy]').forEach((button) => {
        button.addEventListener('click', async () => {
            const value = button.getAttribute('data-copy');
            if (!value) {
                return;
            }
            if (await copyText(value)) {
                const original = button.textContent;
                button.textContent = 'copied';
                button.classList.add('is-copied');
                window.setTimeout(() => {
                    button.textContent = original;
                    button.classList.remove('is-copied');
                }, 1600);
            }
        });
    });

    document.querySelectorAll('pre').forEach((pre) => {
        const code = pre.querySelector('code');
        if (!code) {
            return;
        }

        const button = document.createElement('button');
        button.type = 'button';
        button.className = 'copy-btn';
        button.textContent = 'copy';
        button.setAttribute('aria-label', 'Copy code to clipboard');

        button.addEventListener('click', async () => {
            if (await copyText(code.innerText)) {
                button.textContent = 'copied';
                window.setTimeout(() => {
                    button.textContent = 'copy';
                }, 1600);
            }
        });

        pre.appendChild(button);
    });
})();
