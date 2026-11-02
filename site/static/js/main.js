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

    const header = document.querySelector('.site-header');
    if (header) {
        const updateHeader = () => header.classList.toggle('scrolled', window.scrollY > 8);
        updateHeader();
        window.addEventListener('scroll', updateHeader, { passive: true });
    }

    const canvas = document.getElementById('particle-canvas');
    if (!canvas) {
        return;
    }

    const ctx = canvas.getContext('2d');
    if (!ctx) {
        return;
    }

    const reduceMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    const colors = ['rgba(0, 229, 168, ', 'rgba(56, 189, 248, ', 'rgba(167, 139, 250, '];
    let width = 0;
    let height = 0;
    let particles = [];
    let frame = 0;

    const draw = () => {
        ctx.clearRect(0, 0, width, height);

        for (let i = 0; i < particles.length; i += 1) {
            const p = particles[i];

            for (let j = i + 1; j < particles.length; j += 1) {
                const q = particles[j];
                const dx = p.x - q.x;
                const dy = p.y - q.y;
                const distance = Math.hypot(dx, dy);
                if (distance < 115) {
                    ctx.strokeStyle = `rgba(0, 229, 168, ${(1 - distance / 115) * 0.09})`;
                    ctx.lineWidth = 1;
                    ctx.beginPath();
                    ctx.moveTo(p.x, p.y);
                    ctx.lineTo(q.x, q.y);
                    ctx.stroke();
                }
            }

            ctx.fillStyle = p.color + p.alpha + ')';
            ctx.beginPath();
            ctx.arc(p.x, p.y, p.radius, 0, Math.PI * 2);
            ctx.fill();
        }
    };

    const tick = () => {
        for (const p of particles) {
            p.x += p.vx;
            p.y += p.vy;
            if (p.x < 0 || p.x > width) p.vx *= -1;
            if (p.y < 0 || p.y > height) p.vy *= -1;
        }
        draw();
        frame = window.requestAnimationFrame(tick);
    };

    const resize = () => {
        const rect = canvas.getBoundingClientRect();
        const dpr = Math.min(window.devicePixelRatio || 1, 2);
        width = rect.width;
        height = rect.height;
        canvas.width = Math.round(width * dpr);
        canvas.height = Math.round(height * dpr);
        ctx.setTransform(dpr, 0, 0, dpr, 0, 0);

        const count = Math.min(70, Math.floor(width / 22));
        particles = Array.from({ length: count }, () => ({
            x: Math.random() * width,
            y: Math.random() * height,
            vx: (Math.random() - 0.5) * 0.24,
            vy: (Math.random() - 0.5) * 0.24,
            radius: Math.random() * 1.5 + 0.5,
            color: colors[Math.floor(Math.random() * colors.length)],
            alpha: Math.random() * 0.4 + 0.15,
        }));

        if (reduceMotion) {
            draw();
        }
    };

    resize();
    if (!reduceMotion) {
        tick();
    }

    window.addEventListener('resize', () => {
        window.cancelAnimationFrame(frame);
        resize();
        if (!reduceMotion) {
            tick();
        }
    });
})();
