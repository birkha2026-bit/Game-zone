<!DOCTYPE html>
<html lang="ar" dir="rtl">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0, maximum-scale=1.0, user-scalable=no">
    <title>لعبة نيون أونلاين - تطور جديد</title>
    <style>
        body {
            margin: 0;
            padding: 0;
            overflow: hidden;
            background-color: #0a0a0a;
            font-family: 'Tahoma', sans-serif;
            user-select: none;
            -webkit-user-select: none;
        }
        #canvas-container {
            width: 100vw;
            height: 100vh;
            display: block;
        }
        #ui-layer {
            position: absolute;
            top: 0;
            left: 0;
            width: 100%;
            height: 100%;
            pointer-events: none;
            display: flex;
            flex-direction: column;
            justify-content: space-between;
            padding: 20px;
            box-sizing: border-box;
        }
        .hud-panel {
            background: rgba(0, 0, 0, 0.6);
            border: 2px solid #00ffcc;
            color: #00ffcc;
            padding: 10px 20px;
            border-radius: 8px;
            font-size: 18px;
            font-weight: bold;
            text-shadow: 0 0 5px #00ffcc;
            pointer-events: auto;
        }
        .mobile-controls {
            position: absolute;
            bottom: 20px;
            left: 20px;
            right: 20px;
            height: 150px;
            pointer-events: none;
            display: flex;
            justify-content: space-between;
            align-items: flex-end;
        }
        .joy-area {
            width: 130px;
            height: 130px;
            background: rgba(255, 255, 255, 0.1);
            border: 2px solid rgba(255, 255, 255, 0.3);
            border-radius: 50%;
            pointer-events: auto;
            position: relative;
            touch-action: none;
        }
        .joy-thumb {
            width: 50px;
            height: 50px;
            background: #00ffcc;
            border-radius: 50%;
            position: absolute;
            top: 40px;
            left: 40px;
            box-shadow: 0 0 10px #00ffcc;
        }
        .action-buttons {
            display: flex;
            gap: 15px;
            pointer-events: auto;
        }
        .btn-game {
            width: 70px;
            height: 70px;
            background: rgba(0, 255, 204, 0.2);
            border: 2px solid #00ffcc;
            border-radius: 50%;
            color: #fff;
            font-size: 16px;
            font-weight: bold;
            display: flex;
            align-items: center;
            justify-content: center;
            box-shadow: 0 0 10px rgba(0,255,204,0.5);
            touch-action: manipulation;
        }
        .btn-game:active {
            background: rgba(0, 255, 204, 0.6);
        }
        #crosshair {
            position: absolute;
            top: 50%;
            left: 50%;
            width: 10px;
            height: 10px;
            transform: translate(-50%, -50%);
            border: 2px solid #ff0055;
            border-radius: 50%;
            pointer-events: none;
            box-shadow: 0 0 5px #ff0055;
        }
    </style>
    <script src="https://cdnjs.cloudflare.com/ajax/libs/three.js/r128/three.min.js"></script>
</head>
<body>

    <div id="canvas-container"></div>
    <div id="crosshair"></div>

    <div id="ui-layer">
        <div style="display: flex; justify-content: space-between;">
            <div class="hud-panel">النقاط: <span id="score">0</span></div>
            <div class="hud-panel">الصحة: <span id="health">100</span>%</div>
        </div>

        <div class="mobile-controls">
            <div class="joy-area" id="joyArea">
                <div class="joy-thumb" id="joyThumb"></div>
            </div>
            <div class="action-buttons">
                <div class="btn-game" id="btnJump">قفز</div>
                <div class="btn-game" id="btnShoot" style="background: rgba(255,0,85,0.3); border-color: #ff0055;">إطلاق</div>
            </div>
        </div>
    </div>

    <script>
        const container = document.getElementById('canvas-container');
        const scene = new THREE.Scene();
        scene.background = new THREE.Color(0x050505);
        scene.fog = new THREE.FogExp2(0x050505, 0.03);

        const camera = new THREE.PerspectiveCamera(75, window.innerWidth / window.innerHeight, 0.1, 1000);
        const renderer = new THREE.WebGLRenderer({ antialias: true });
        renderer.setSize(window.innerWidth, window.innerHeight);
        renderer.setPixelRatio(window.devicePixelRatio);
        container.appendChild(renderer.domElement);

        const ambientLight = new THREE.AmbientLight(0xffffff, 0.6);
        scene.add(ambientLight);

        const pointLight = new THREE.PointLight(0x00ffcc, 2, 50);
        pointLight.position.set(0, 10, 0);
        scene.add(pointLight);

        const floorGeo = new THREE.PlaneGeometry(100, 100);
        const floorMat = new THREE.MeshStandardMaterial({ color: 0x111111, roughness: 0.8 });
        const floor = new THREE.Mesh(floorGeo, floorMat);
        floor.rotation.x = -Math.PI / 2;
        scene.add(floor);

        const boxMat = new THREE.MeshStandardMaterial({ color: 0x00ffcc, emissive: 0x004433 });
        for (let i = 0; i < 20; i++) {
            const size = Math.random() * 3 + 2;
            const boxGeo = new THREE.BoxGeometry(size, size, size);
            const box = new THREE.Mesh(boxGeo, boxMat);
            box.position.set(
                (Math.random() - 0.5) * 80,
                size / 2,
                (Math.random() - 0.5) * 80
            );
            scene.add(box);
        }

        camera.position.set(0, 2, 10);

        let moveForward = false;
        let moveBackward = false;
        let moveLeft = false;
        let moveRight = false;

        const joyArea = document.getElementById('joyArea');
        const joyThumb = document.getElementById('joyThumb');
        let joyCenter = { x: 0, y: 0 };
        let touchId = null;

        joyArea.addEventListener('touchstart', (e) => {
            const touch = e.changedTouches[0];
            touchId = touch.identifier;
            const rect = joyArea.getBoundingClientRect();
            joyCenter = { x: rect.left + rect.width / 2, y: rect.top + rect.height / 2 };
        });

        joyArea.addEventListener('touchmove', (e) => {
            for (let i = 0; i < e.changedTouches.length; i++) {
                const touch = e.changedTouches[i];
                if (touch.identifier === touchId) {
                    let dx = touch.clientX - joyCenter.x;
                    let dy = touch.clientY - joyCenter.y;
                    let dist = Math.hypot(dx, dy);
                    let maxDist = 40;

                    if (dist > maxDist) {
                        dx = (dx / dist) * maxDist;
                        dy = (dy / dist) * maxDist;
                    }

                    joyThumb.style.transform = `translate(${dx}px, ${dy}px)`;

                    moveForward = dy < -10;
                    moveBackward = dy > 10;
                    moveLeft = dx < -10;
                    moveRight = dx > 10;
                }
            }
        });

        joyArea.addEventListener('touchend', (e) => {
            for (let i = 0; i < e.changedTouches.length; i++) {
                if (e.changedTouches[i].identifier === touchId) {
                    touchId = null;
                    joyThumb.style.transform = `translate(0px, 0px)`;
                    moveForward = moveBackward = moveLeft = moveRight = false;
                }
            }
        });

        document.getElementById('btnJump').addEventListener('click', () => {
            camera.position.y += 1.5;
            setTimeout(() => { camera.position.y -= 1.5; }, 300);
        });

        document.getElementById('btnShoot').addEventListener('click', () => {
            const crosshair = document.getElementById('crosshair');
            crosshair.style.borderColor = '#00ffcc';
            setTimeout(() => {
                crosshair.style.borderColor = '#ff0055';
            }, 100);
        });

        function animate() {
            requestAnimationFrame(animate);

            const speed = 0.1;
            if (moveForward) camera.translateZ(-speed);
            if (moveBackward) camera.translateZ(speed);
            if (moveLeft) camera.translateX(-speed);
            if (moveRight) camera.translateX(speed);

            renderer.render(scene, camera);
        }

        animate();

        window.addEventListener('resize', () => {
            camera.aspect = window.innerWidth / window.innerHeight;
            camera.updateProjectionMatrix();
            renderer.setSize(window.innerWidth, window.innerHeight);
        });
    </script>
</body>
</html>
