package http

// HTMLPage содержит интерфейс панели управления камерами
const HTMLPage = `
<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>WebCam Streamer</title>
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
            max-width: 800px;
            margin: 0 auto;
            background: #2d2d2d;
            padding: 30px;
            border-radius: 8px;
            box-shadow: 0 4px 15px rgba(0,0,0,0.5);
        }
        h1 { color: #00add8; font-size: 24px; margin-bottom: 25px; }
        label { font-size: 16px; margin-right: 10px; }
        select { 
            padding: 10px; 
            font-size: 16px; 
            background: #3c3c3c; 
            color: #fff; 
            border: 1px solid #555; 
            border-radius: 4px;
            min-width: 200px;
        }
        button { 
            padding: 10px 20px; 
            font-size: 16px; 
            background: #00add8; 
            color: #fff; 
            border: none; 
            border-radius: 4px; 
            cursor: pointer;
            font-weight: bold;
            margin-left: 10px;
            transition: background 0.2s;
        }
        button:hover { background: #008eb3; }
        .video-box { margin-top: 30px; background: #000; border-radius: 6px; overflow: hidden; line-height: 0; }
        #view { max-width: 100%; height: auto; display: none; }
        .placeholder { padding: 100px 20px; color: #777; font-style: italic; }
    </style>
</head>
<body>
    <div class="container">
        <h1>Управление USB-камерами (Go Streamer)</h1>
        
        <div class="controls">
            <label for="cameraSelect">Доступные устройства:</label>
            <select id="cameraSelect">
                <option value="">Поиск камер...</option>
            </select>
            <button onclick="startStream()">Трансляция</button>
        </div>
        
        <div class="video-box">
            <div id="placeholder" class="placeholder">Выберите камеру и нажмите кнопку «Трансляция»</div>
            <img id="view" src="" alt="Поток камеры" />
        </div>
    </div>

    <script>
        // Загрузка списка камер с бэкенда при старте страницы
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
                    opt.value = cam.path; // Передаем системный путь (/dev/video0)
                    opt.textContent = cam.name; // Показываем понятное имя
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
            const viewImg = document.getElementById('view');
            const placeholder = document.getElementById('placeholder');
            
            if (!path) return;
            
            placeholder.style.display = 'none';
            viewImg.style.display = 'inline-block';
            
            // Задаем src на эндпоинт стриминга с query-параметром dev
            viewImg.src = '/stream?dev=' + encodeURIComponent(path);
        }

        // Инициализация
        loadCameras();
    </script>
</body>
</html>
`
