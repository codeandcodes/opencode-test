// Tetris Game - SRS Rotation System
// ==================================

// SRS Wall Kick Tables
// Standard SRS defines different kick tables for I-piece vs other pieces (JLSTZ)
// Each entry is [x, y] offset to apply when rotation would cause wall collision
// Order: 0→R, R→2, 2→L, L→0 for clockwise; reverse for counter-clockwise

const SRS_KICKS_JLSTZ = [
    // 0→R, R→2, 2→L, L→0 (clockwise rotations)
    [[0, 0], [-1, 0], [-1, 1], [0, -2], [-1, -2]],  // 0→R
    [[0, 0], [1, 0], [1, -1], [0, 2], [1, 2]],      // R→2
    [[0, 0], [1, 0], [1, 1], [0, -2], [1, -2]],     // 2→L
    [[0, 0], [-1, 0], [-1, -1], [0, 2], [-1, 2]]    // L→0
];

const SRS_KICKS_I = [
    // I-piece uses different kick tables
    [[0, 0], [-2, 0], [1, 0], [-2, -1], [1, 2]],    // 0→R
    [[0, 0], [-1, 0], [2, 0], [-1, 2], [2, -1]],    // R→2
    [[0, 0], [2, 0], [-1, 0], [2, 1], [-1, -2]],    // 2→L
    [[0, 0], [1, 0], [-2, 0], [1, -2], [-2, 2]]     // L→0
];

// Gravity curve: frames per drop decreases with level
// Level 1: 48 frames (0.8s at 60fps), Level 20: 5 frames (~0.08s)
// Formula: frames = max(5, floor(48 * 0.8^(level-1)))
function getGravityFrames(level) {
    if (level >= 20) return 5;
    return Math.max(5, Math.floor(48 * Math.pow(0.8, level - 1)));
}

// Tetromino definitions - each piece has 4 rotation states
// Each state is a 4x4 grid (or 2x2 for O) with cell positions
const TETROMINOES = {
    I: {
        color: '#00f0f0',
        shapes: [
            [[0,1], [1,1], [2,1], [3,1]],
            [[2,0], [2,1], [2,2], [2,3]],
            [[0,2], [1,2], [2,2], [3,2]],
            [[1,0], [1,1], [1,2], [1,3]]
        ]
    },
    J: {
        color: '#0000f0',
        shapes: [
            [[0,0], [0,1], [1,1], [2,1]],
            [[1,0], [2,0], [1,1], [1,2]],
            [[0,2], [1,2], [2,2], [2,1]],
            [[1,0], [1,1], [0,2], [1,2]]
        ]
    },
    L: {
        color: '#f0a000',
        shapes: [
            [[2,0], [0,1], [1,1], [2,1]],
            [[1,0], [1,1], [1,2], [2,2]],
            [[0,1], [0,2], [1,2], [2,2]],
            [[0,0], [1,0], [1,1], [1,2]]
        ]
    },
    O: {
        color: '#f0f000',
        shapes: [
            [[1,0], [2,0], [1,1], [2,1]],
            [[1,0], [2,0], [1,1], [2,1]],
            [[1,0], [2,0], [1,1], [2,1]],
            [[1,0], [2,0], [1,1], [2,1]]
        ]
    },
    S: {
        color: '#00f000',
        shapes: [
            [[1,0], [2,0], [0,1], [1,1]],
            [[1,0], [1,1], [2,1], [2,2]],
            [[1,1], [2,1], [0,2], [1,2]],
            [[0,0], [0,1], [1,1], [1,2]]
        ]
    },
    T: {
        color: '#a000f0',
        shapes: [
            [[1,0], [0,1], [1,1], [2,1]],
            [[1,0], [1,1], [2,1], [1,2]],
            [[0,1], [1,1], [2,1], [1,2]],
            [[1,0], [0,1], [1,1], [1,2]]
        ]
    },
    Z: {
        color: '#f00000',
        shapes: [
            [[0,0], [1,0], [1,1], [2,1]],
            [[2,0], [1,1], [2,1], [1,2]],
            [[0,1], [1,1], [1,2], [2,2]],
            [[1,0], [0,1], [1,1], [0,2]]
        ]
    }
};

const PIECE_NAMES = ['I', 'J', 'L', 'O', 'S', 'T', 'Z'];

// Game constants
const BOARD_WIDTH = 10;
const BOARD_HEIGHT = 20;
const CELL_SIZE = 30;
const LOCK_DELAY = 500;
const MAX_LOCK_RESETS = 15;

// Scoring: base points multiplied by level
const LINE_SCORES = { 1: 100, 2: 300, 3: 500, 4: 800 };
const COMBO_BONUS = 50;

// Game state
let board = [];
let currentPiece = null;
let nextQueue = [];
let holdPiece = null;
let canHold = true;
let score = 0;
let lines = 0;
let level = 1;
let combo = -1;
let gameOver = false;
let paused = false;
let gameStarted = false;

// Timing
let dropCounter = 0;
let lockDelayCounter = 0;
let lockResets = 0;
let lastTime = 0;

// Canvas
const gameCanvas = document.getElementById('gameCanvas');
const ctx = gameCanvas.getContext('2d');
const nextCanvas = document.getElementById('nextCanvas');
const nextCtx = nextCanvas.getContext('2d');
const holdCanvas = document.getElementById('holdCanvas');
const holdCtx = holdCanvas.getContext('2d');

// Input handling
const keys = {};
let repeatTimers = {};

// 7-bag randomizer
let currentBag = [];
function getNextPieceFromBag() {
    if (currentBag.length === 0) {
        // Create new shuffled bag with all 7 pieces
        currentBag = [...PIECE_NAMES];
        // Fisher-Yates shuffle
        for (let i = currentBag.length - 1; i > 0; i--) {
            const j = Math.floor(Math.random() * (i + 1));
            [currentBag[i], currentBag[j]] = [currentBag[j], currentBag[i]];
        }
    }
    return currentBag.pop();
}

function initBoard() {
    board = [];
    for (let y = 0; y < BOARD_HEIGHT; y++) {
        board[y] = [];
        for (let x = 0; x < BOARD_WIDTH; x++) {
            board[y][x] = null;
        }
    }
}

function createPiece(name) {
    const piece = TETROMINOES[name];
    return {
        name: name,
        color: piece.color,
        shapes: piece.shapes,
        rotation: 0,
        x: Math.floor(BOARD_WIDTH / 2) - 2,
        y: 0
    };
}

function spawnPiece() {
    // Fill next queue if needed
    while (nextQueue.length < 4) {
        nextQueue.push(getNextPieceFromBag());
    }
    
    const pieceName = nextQueue.shift();
    currentPiece = createPiece(pieceName);
    canHold = true;
    lockResets = 0;
    lockDelayCounter = 0;
    
    // Check for game over (spawn position blocked)
    if (collides(currentPiece, 0, 0)) {
        gameOver = true;
        showGameOver();
    }
}

function collides(piece, offsetX, offsetY, newRotation = null) {
    const rotation = newRotation !== null ? newRotation : piece.rotation;
    const shape = piece.shapes[rotation];
    
    for (const [cx, cy] of shape) {
        const x = piece.x + cx + offsetX;
        const y = piece.y + cy + offsetY;
        
        // Check bounds
        if (x < 0 || x >= BOARD_WIDTH || y >= BOARD_HEIGHT) {
            return true;
        }
        
        // Check board (only if y >= 0, pieces can be partially above board)
        if (y >= 0 && board[y][x] !== null) {
            return true;
        }
    }
    return false;
}

function rotate(direction) {
    if (!currentPiece || paused || gameOver) return;
    
    const pieceType = currentPiece.name;
    if (pieceType === 'O') return; // O piece doesn't rotate
    
    const newRotation = (currentPiece.rotation + direction + 4) % 4;
    
    // Get kick tables based on piece type
    const kickTable = pieceType === 'I' ? SRS_KICKS_I : SRS_KICKS_JLSTZ;
    const kickIndex = direction > 0 
        ? currentPiece.rotation 
        : (currentPiece.rotation + 3) % 4;
    const kicks = kickTable[kickIndex];
    
    // Try each kick position
    for (const [kickX, kickY] of kicks) {
        if (!collides(currentPiece, kickX, kickY, newRotation)) {
            currentPiece.rotation = newRotation;
            currentPiece.x += kickX;
            currentPiece.y += kickY;
            lockResets = 0; // Reset lock delay on rotation
            return;
        }
    }
}

function move(dx, dy) {
    if (!currentPiece || paused || gameOver) return false;
    
    if (!collides(currentPiece, dx, dy)) {
        currentPiece.x += dx;
        currentPiece.y += dy;
        if (dx !== 0) {
            lockResets = 0; // Reset lock delay on horizontal movement
        }
        return true;
    }
    return false;
}

function softDrop() {
    if (!currentPiece || paused || gameOver) return;
    
    if (move(0, 1)) {
        score += 1; // 1 point per cell
        updateDisplay();
    } else {
        // Start or continue lock delay
        if (lockDelayCounter === 0) {
            lockDelayCounter = LOCK_DELAY;
        }
    }
}

function hardDrop() {
    if (!currentPiece || paused || gameOver) return;
    
    let dropDistance = 0;
    while (move(0, 1)) {
        dropDistance++;
    }
    
    score += dropDistance * 2; // 2 points per cell
    lockPiece();
    updateDisplay();
}

function hold() {
    if (!currentPiece || !canHold || paused || gameOver) return;
    
    canHold = false;
    
    if (holdPiece === null) {
        holdPiece = currentPiece.name;
        spawnPiece();
    } else {
        const temp = holdPiece;
        holdPiece = currentPiece.name;
        currentPiece = createPiece(temp);
        currentPiece.rotation = 0;
        currentPiece.x = Math.floor(BOARD_WIDTH / 2) - 2;
        currentPiece.y = 0;
    }
}

function getGhostY() {
    if (!currentPiece) return 0;
    
    let ghostY = currentPiece.y;
    while (!collides(currentPiece, 0, ghostY - currentPiece.y + 1)) {
        ghostY++;
    }
    return ghostY;
}

function lockPiece() {
    const shape = currentPiece.shapes[currentPiece.rotation];
    
    for (const [cx, cy] of shape) {
        const x = currentPiece.x + cx;
        const y = currentPiece.y + cy;
        
        if (y >= 0 && y < BOARD_HEIGHT && x >= 0 && x < BOARD_WIDTH) {
            board[y][x] = currentPiece.color;
        }
    }
    
    clearLines();
    spawnPiece();
}

function clearLines() {
    let linesCleared = 0;
    
    for (let y = BOARD_HEIGHT - 1; y >= 0; y--) {
        if (board[y].every(cell => cell !== null)) {
            // Remove the line
            board.splice(y, 1);
            // Add empty line at top
            const emptyLine = [];
            for (let x = 0; x < BOARD_WIDTH; x++) {
                emptyLine[x] = null;
            }
            board.unshift(emptyLine);
            linesCleared++;
            y++; // Check same index again
        }
    }
    
    if (linesCleared > 0) {
        // Update combo
        combo++;
        
        // Calculate score
        const baseScore = LINE_SCORES[linesCleared];
        score += baseScore * level;
        
        // Combo bonus: 50 x combo x level for consecutive line-clearing drops
        if (combo > 0) {
            score += COMBO_BONUS * combo * level;
        }
        
        // Update lines and level
        lines += linesCleared;
        level = 1 + Math.floor(lines / 10);
    } else {
        combo = -1;
    }
}

function updateDisplay() {
    document.getElementById('score').textContent = score;
    document.getElementById('level').textContent = level;
    document.getElementById('lines').textContent = lines;
}

function drawCell(cx, cy, color, ctx, cellSize = CELL_SIZE) {
    const x = cx * cellSize;
    const y = cy * cellSize;
    
    // Main cell
    ctx.fillStyle = color;
    ctx.fillRect(x, y, cellSize, cellSize);
    
    // Highlight (top-left)
    ctx.fillStyle = 'rgba(255, 255, 255, 0.3)';
    ctx.fillRect(x, y, cellSize, 3);
    ctx.fillRect(x, y, 3, cellSize);
    
    // Shadow (bottom-right)
    ctx.fillStyle = 'rgba(0, 0, 0, 0.3)';
    ctx.fillRect(x + cellSize - 3, y, 3, cellSize);
    ctx.fillRect(x, y + cellSize - 3, cellSize, 3);
    
    // Border
    ctx.strokeStyle = 'rgba(0, 0, 0, 0.5)';
    ctx.strokeRect(x, y, cellSize, cellSize);
}

function drawEmptyCell(cx, cy) {
    const x = cx * CELL_SIZE;
    const y = cy * CELL_SIZE;
    
    ctx.strokeStyle = 'rgba(255, 255, 255, 0.05)';
    ctx.strokeRect(x, y, CELL_SIZE, CELL_SIZE);
}

function drawBoard() {
    // Clear canvas
    ctx.fillStyle = '#0a0a15';
    ctx.fillRect(0, 0, gameCanvas.width, gameCanvas.height);
    
    // Draw grid
    for (let y = 0; y < BOARD_HEIGHT; y++) {
        for (let x = 0; x < BOARD_WIDTH; x++) {
            drawEmptyCell(x, y);
        }
    }
    
    // Draw locked pieces
    for (let y = 0; y < BOARD_HEIGHT; y++) {
        for (let x = 0; x < BOARD_WIDTH; x++) {
            if (board[y][x]) {
                drawCell(x, y, board[y][x], ctx);
            }
        }
    }
}

function drawPiece(piece, ctx, ghost = false) {
    const shape = piece.shapes[piece.rotation];
    const alpha = ghost ? 0.3 : 1;
    
    for (const [cx, cy] of shape) {
        const x = (piece.x + cx) * CELL_SIZE;
        const y = (piece.y + cy) * CELL_SIZE;
        
        if (!ghost && piece.y + cy < 0) continue; // Don't draw above board
        
        ctx.globalAlpha = alpha;
        ctx.fillStyle = piece.color;
        ctx.fillRect(x, y, CELL_SIZE, CELL_SIZE);
        
        if (!ghost) {
            // Highlight
            ctx.fillStyle = 'rgba(255, 255, 255, 0.3)';
            ctx.fillRect(x, y, CELL_SIZE, 3);
            ctx.fillRect(x, y, 3, CELL_SIZE);
            
            // Shadow
            ctx.fillStyle = 'rgba(0, 0, 0, 0.3)';
            ctx.fillRect(x + CELL_SIZE - 3, y, 3, CELL_SIZE);
            ctx.fillRect(x, y + CELL_SIZE - 3, CELL_SIZE, 3);
            
            // Border
            ctx.strokeStyle = 'rgba(0, 0, 0, 0.5)';
            ctx.strokeRect(x, y, CELL_SIZE, CELL_SIZE);
        }
        ctx.globalAlpha = 1;
    }
}

function drawGhost() {
    if (!currentPiece) return;
    
    const ghostY = getGhostY();
    const shape = currentPiece.shapes[currentPiece.rotation];
    
    for (const [cx, cy] of shape) {
        const x = (currentPiece.x + cx) * CELL_SIZE;
        const y = (ghostY + cy) * CELL_SIZE;
        
        if (ghostY + cy < 0) continue;
        
        ctx.globalAlpha = 0.3;
        ctx.fillStyle = currentPiece.color;
        ctx.fillRect(x, y, CELL_SIZE, CELL_SIZE);
        ctx.strokeStyle = currentPiece.color;
        ctx.lineWidth = 2;
        ctx.strokeRect(x, y, CELL_SIZE, CELL_SIZE);
        ctx.globalAlpha = 1;
        ctx.lineWidth = 1;
    }
}

function drawNext() {
    nextCtx.fillStyle = 'rgba(0, 0, 0, 0.3)';
    nextCtx.fillRect(0, 0, nextCanvas.width, nextCanvas.height);
    
    // Draw next 3 pieces
    for (let i = 0; i < 3 && i < nextQueue.length; i++) {
        const pieceName = nextQueue[i];
        const piece = TETROMINOES[pieceName];
        const shape = piece.shapes[0];
        
        // Center the piece in the canvas
        let minX = Infinity, maxX = -Infinity, minY = Infinity, maxY = -Infinity;
        for (const [x, y] of shape) {
            minX = Math.min(minX, x);
            maxX = Math.max(maxX, x);
            minY = Math.min(minY, y);
            maxY = Math.max(maxY, y);
        }
        
        const pieceWidth = maxX - minX + 1;
        const pieceHeight = maxY - minY + 1;
        const offsetX = Math.floor((4 - pieceWidth) / 2) - minX;
        const offsetY = 0;
        
        const startY = 10 + i * 70;
        const cellSize = 20;
        
        for (const [cx, cy] of shape) {
            const x = (cx + offsetX) * cellSize;
            const y = (cy + offsetY + startY) * cellSize;
            
            nextCtx.fillStyle = piece.color;
            nextCtx.fillRect(x, y, cellSize, cellSize);
            nextCtx.strokeStyle = 'rgba(0, 0, 0, 0.5)';
            nextCtx.strokeRect(x, y, cellSize, cellSize);
        }
    }
}

function drawHold() {
    holdCtx.fillStyle = 'rgba(0, 0, 0, 0.3)';
    holdCtx.fillRect(0, 0, holdCanvas.width, holdCanvas.height);
    
    if (!holdPiece) return;
    
    const piece = TETROMINOES[holdPiece];
    const shape = piece.shapes[0];
    
    // Center the piece
    let minX = Infinity, maxX = -Infinity, minY = Infinity, maxY = -Infinity;
    for (const [x, y] of shape) {
        minX = Math.min(minX, x);
        maxX = Math.max(maxX, x);
        minY = Math.min(minY, y);
        maxY = Math.max(maxY, y);
    }
    
    const pieceWidth = maxX - minX + 1;
    const pieceHeight = maxY - minY + 1;
    const offsetX = Math.floor((4 - pieceWidth) / 2) - minX;
    const offsetY = Math.floor((4 - pieceHeight) / 2) - minY;
    
    const cellSize = 20;
    
    for (const [cx, cy] of shape) {
        const x = (cx + offsetX) * cellSize;
        const y = (cy + offsetY) * cellSize;
        
        if (!canHold) {
            holdCtx.fillStyle = 'rgba(100, 100, 100, 0.5)';
        } else {
            holdCtx.fillStyle = piece.color;
        }
        holdCtx.fillRect(x, y, cellSize, cellSize);
        holdCtx.strokeStyle = 'rgba(0, 0, 0, 0.5)';
        holdCtx.strokeRect(x, y, cellSize, cellSize);
    }
}

function draw() {
    drawBoard();
    
    if (currentPiece) {
        drawGhost();
        drawPiece(currentPiece, ctx);
    }
    
    drawNext();
    drawHold();
}

function update(time = 0) {
    if (gameOver || paused || !gameStarted) {
        requestAnimationFrame(update);
        return;
    }
    
    const deltaTime = time - lastTime;
    lastTime = time;
    
    // Gravity
    const gravityFrames = getGravityFrames(level);
    dropCounter += deltaTime;
    
    if (dropCounter >= (1000 / 60) * gravityFrames) {
        dropCounter = 0;
        
        if (!move(0, 1)) {
            // Start or continue lock delay
            if (lockDelayCounter === 0) {
                lockDelayCounter = LOCK_DELAY;
            } else {
                lockDelayCounter -= deltaTime;
                if (lockDelayCounter <= 0) {
                    lockPiece();
                }
            }
        } else {
            // Successfully moved down - reset lock delay
            lockDelayCounter = 0;
        }
    }
    
    draw();
    requestAnimationFrame(update);
}

function startGame() {
    initBoard();
    score = 0;
    lines = 0;
    level = 1;
    combo = -1;
    gameOver = false;
    paused = false;
    gameStarted = true;
    holdPiece = null;
    canHold = true;
    nextQueue = [];
    currentBag = [];
    currentPiece = null;
    dropCounter = 0;
    lockDelayCounter = 0;
    lockResets = 0;
    lastTime = performance.now();
    
    updateDisplay();
    hideOverlay();
    
    spawnPiece();
    draw();
    requestAnimationFrame(update);
}

function togglePause() {
    if (!gameStarted || gameOver) return;
    
    paused = !paused;
    
    if (paused) {
        showOverlay('<h2 class="pause-text">PAUSED</h2><p>Press P to resume</p>');
    } else {
        hideOverlay();
        lastTime = performance.now();
        requestAnimationFrame(update);
    }
}

function showOverlay(html) {
    const overlay = document.getElementById('overlay');
    const content = document.getElementById('overlayContent');
    content.innerHTML = html;
    overlay.classList.remove('hidden');
}

function hideOverlay() {
    document.getElementById('overlay').classList.add('hidden');
}

function showGameOver() {
    saveHighScore(score);
    const highScores = getHighScores();
    
    let html = '<h1>GAME OVER</h1>';
    html += `<p>Score: ${score}</p>`;
    html += `<p>Level: ${level}</p>`;
    html += `<p>Lines: ${lines}</p>`;
    
    html += '<div class="high-scores"><h3>TOP 5</h3><ol>';
    highScores.forEach((entry, i) => {
        html += `<li>
            <span><span class="rank">#${i + 1}</span> ${entry.score}</span>
            <span class="date">${entry.date}</span>
        </li>`;
    });
    html += '</ol></div>';
    
    html += '<button class="button" onclick="startGame()">PLAY AGAIN</button>';
    
    showOverlay(html);
}

// High score management
const HIGH_SCORES_KEY = 'tetris_high_scores';

function getHighScores() {
    try {
        const stored = localStorage.getItem(HIGH_SCORES_KEY);
        return stored ? JSON.parse(stored) : [];
    } catch (e) {
        return [];
    }
}

function saveHighScore(newScore) {
    const highScores = getHighScores();
    const date = new Date().toLocaleDateString();
    
    highScores.push({ score: newScore, date: date });
    highScores.sort((a, b) => b.score - a.score);
    
    // Keep top 5
    const top5 = highScores.slice(0, 5);
    
    try {
        localStorage.setItem(HIGH_SCORES_KEY, JSON.stringify(top5));
    } catch (e) {
        // localStorage might be disabled
    }
}

// Input handling
document.addEventListener('keydown', (e) => {
    if (!gameStarted && e.code !== 'KeyP') {
        // Start game on first keypress (except P which shows instructions)
        if (e.code === 'Enter' || e.code === 'Space') {
            startGame();
            return;
        }
    }
    
    switch (e.code) {
        case 'ArrowLeft':
        case 'KeyA':
            e.preventDefault();
            if (gameStarted && !gameOver && !paused) {
                move(-1, 0);
                resetRepeatTimer('left');
            }
            break;
            
        case 'ArrowRight':
        case 'KeyD':
            e.preventDefault();
            if (gameStarted && !gameOver && !paused) {
                move(1, 0);
                resetRepeatTimer('right');
            }
            break;
            
        case 'ArrowDown':
        case 'KeyS':
            e.preventDefault();
            if (gameStarted && !gameOver && !paused) {
                softDrop();
                resetRepeatTimer('down');
            }
            break;
            
        case 'ArrowUp':
        case 'KeyW':
            e.preventDefault();
            if (gameStarted && !gameOver && !paused) {
                rotate(1);
            }
            break;
            
        case 'KeyQ':
            e.preventDefault();
            if (gameStarted && !gameOver && !paused) {
                rotate(-1);
            }
            break;
            
        case 'KeyE':
            e.preventDefault();
            if (gameStarted && !gameOver && !paused) {
                rotate(-1);
            }
            break;
            
        case 'Space':
            e.preventDefault();
            if (gameStarted && !gameOver && !paused) {
                hardDrop();
            }
            break;
            
        case 'KeyC':
        case 'ShiftLeft':
        case 'ShiftRight':
            e.preventDefault();
            hold();
            break;
            
        case 'KeyP':
            e.preventDefault();
            if (!gameStarted) {
                showOverlay('<h2>TETRIS</h2><p>Press ENTER or SPACE to start</p>');
            } else {
                togglePause();
            }
            break;
    }
    
    keys[e.code] = true;
});

document.addEventListener('keyup', (e) => {
    keys[e.code] = false;
    
    if (e.code === 'ArrowLeft' || e.code === 'KeyA') {
        cancelRepeatTimer('left');
    } else if (e.code === 'ArrowRight' || e.code === 'KeyD') {
        cancelRepeatTimer('right');
    } else if (e.code === 'ArrowDown' || e.code === 'KeyS') {
        cancelRepeatTimer('down');
    }
});

// Horizontal auto-repeat
const REPEAT_DELAY = 150;
const REPEAT_INTERVAL = 50;

function resetRepeatTimer(key) {
    cancelRepeatTimer(key);
    repeatTimers[key] = {
        initial: setTimeout(() => {
            startRepeat(key);
        }, REPEAT_DELAY)
    };
}

function startRepeat(key) {
    repeatTimers[key].interval = setInterval(() => {
        if (!keys[key.replace('repeat', '')]) {
            cancelRepeatTimer(key);
            return;
        }
        
        const actualKey = key === 'left' ? 'ArrowLeft' : 
                         key === 'right' ? 'ArrowRight' : 'ArrowDown';
        
        if (gameStarted && !gameOver && !paused) {
            if (key === 'left') move(-1, 0);
            else if (key === 'right') move(1, 0);
            else softDrop();
        }
    }, REPEAT_INTERVAL);
}

function cancelRepeatTimer(key) {
    if (repeatTimers[key]) {
        clearTimeout(repeatTimers[key].initial);
        clearInterval(repeatTimers[key].interval);
        delete repeatTimers[key];
    }
}

// Show initial screen
showOverlay('<h2>TETRIS</h2><p>Press ENTER or SPACE to start</p><p>Press P for controls</p>');
