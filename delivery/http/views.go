package http

// HTMLPage содержит интерфейс панели управления камерами
const HTMLPage = `
<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Go WebCam Streamer (Clear Architecture)</title>
    <style>
        body { 
            font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; 
            text-align: center; 
            background: #1e1e1e; 
            color: #f5f5f5; 
            margin: 0;
            padding: 20px;
        }
        .container {
            max-width: 900px;
            margin: 0 auto;
            background: #2d2d2d;
            padding: 30px;
            border-radius: 8px;
            box-shadow: 0 4px 15px rgba(0,0,0,0.5);
        }
        h1 { color: #00add8; font-size: 24px; margin-bottom: 25px; }
        .controls-grid {
            display: flex;
            flex-wrap: wrap;
            justify-content: center;
            gap: 15px;
            align-items: center;
            margin-bottom: 25px;
        }
        .control-group {
            display: flex;
            flex-direction: column;
            align-items: flex-start;
        }
        label { font-size: 14px; margin-bottom: 5px; color: #aaa; }
        select { 
            padding: 10px; 
            font-size: 15px; 
            background: #3c3c3c; 
            color: #fff; 
            border: 1px solid #555; 
            border-radius: 4px;
            min-width: 160px;
        }
        button { 
            padding: 10px 25px; 
            font-size: 15px; 
            background: #00add8; 
            color: #fff; 
            border: none; 
            border-radius: 4px; 
            cursor: pointer;
            font-weight: bold;
            transition: background 0.2s;
            align-self: flex-end;
            height: 40px;
        }
        button:hover { background: #008eb3; }
        .video-box { margin-top: 20px; background: #000; border-radius: 6px; overflow: hidden; line-height: 0; min-height: 200px;}
        #view { max-width: 100%; height: auto; display: none; }
        .placeholder { padding: 100px 20px; color: #777; font-style: italic; }
    </style>
</head>
<body>
    <div class="container">
        <h1>Управление USB-камерами (Go Streamer)</h1>
        
        <div class="controls-grid">
            <div class="control-group">
                <label for="cameraSelect">Устройство:</label>
                <select id="cameraSelect">
                    <option value="">Поиск камер...</option>
                </select>
            </div>

            <div class="control-group">
                <label for="resolutionSelect">Разрешение:</label>
                <select id="resolutionSelect">
                    <option value="640x480">640 x 480 (SD)</option>
                    <option value="1280x720" selected>1280 x 720 (HD)</option>
                    <option value="1920x1080">1920 x 1080 (Full HD)</option>
                    <option value="320x240">320 x 240 (Low)</option>
                </select>
            </div>

            <div class="control-group">
                <label for="fpsSelect">Частота кадров (FPS):</label>
                <select id="fpsSelect">
                    <option value="15">15 FPS</option>
                    <option value="30" selected>30 FPS</option>
                    <option value="60">60 FPS</option>
                </select>
            </div>

            <button onclick="startStream()">Запустить трансляцию</button>
        </div>
        
        <div class="video-box">
            <div id="placeholder" class="placeholder">Выберите параметры и нажмите «Запустить трансляцию»</div>
            <img id="view" src="" alt="Поток камеры" />
        </div>
    </div>

    <script>
        async function loadCameras() {
            const select = document.getElementById('cameraSelect');
            try {
                const res = await fetch('/api/cameras');
                if (!res.ok) throw new Error('Ошибка сервера');
                
                const cameras = await res.json();
                select.innerHTML = '';
                
                if (!cameras || cameras.length === 0) {
                    select.innerHTML = '<option value="">Камеры не найдены</option>';
                    return;
                }
                
                cameras.forEach(cam => {
                    let opt = document.createElement('option');
                    opt.value = cam.path;
                    opt.textContent = cam.name;
                    select.appendChild(opt);
                });
            } catch (err) {
                console.error(err);
                select.innerHTML = '<option value="">Ошибка загрузки списка</option>';
            }
        }

        // Переключение тега img на URL стрима
        function startStream() {
            const path = document.getElementById('cameraSelect').value;
            const resValue = document.getElementById('resolutionSelect').value;
            const fps = document.getElementById('fpsSelect').value;
            
            const viewImg = document.getElementById('view');
            const placeholder = document.getElementById('placeholder');
            
            if (!path || !resValue) return;
            
            // Разделяем разрешение (например, "1280x720" -> width=1280, height=720)
            const [width, height] = resValue.split('x');
            
            placeholder.style.display = 'none';
            viewImg.style.display = 'inline-block';
            
            // Собираем полный URL со всеми параметрами качества
            const streamUrl = '/stream?dev=' + encodeURIComponent(path) + '&w=' + width + '&h=' + height + '&fps=' + fps;
            viewImg.src = streamUrl;
        }

        // Инициализация
        loadCameras();
    </script>
</body>
</html>
`
