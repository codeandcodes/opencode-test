const boardElement = document.getElementById('game-board');
const BOARD_SIZE = 8;

// Game state
let board = [];
let selectedPiece = null;
let currentPlayer = 'red'; // 'red' or 'black'

// Initialize board
function initializeBoard() {
    board = Array(BOARD_SIZE).fill(null).map(() => Array(BOARD_SIZE).fill(null));

    // Place pieces
    for (let row = 0; row < BOARD_SIZE; row++) {
        for (let col = 0; col < BOARD_SIZE; col++) {
            if ((row + col) % 2 === 1) { // Dark squares
                if (row < 3) {
                    board[row][col] = { type: 'piece', color: 'black' };
                } else if (row > BOARD_SIZE - 4) {
                    board[row][col] = { type: 'piece', color: 'red' };
                }
            }
        }
    }
}

// Render board
function renderBoard() {
    boardElement.innerHTML = ''; // Clear previous board

    for (let row = 0; row < BOARD_SIZE; row++) {
        for (let col = 0; col < BOARD_SIZE; col++) {
            const square = document.createElement('div');
            square.classList.add('square', (row + col) % 2 === 0 ? 'light' : 'dark');
            square.dataset.row = row;
            square.dataset.col = col;

            const piece = board[row][col];
            if (piece) {
                const pieceElement = document.createElement('div');
                pieceElement.classList.add('piece', piece.color);
                if (piece.isKing) {
                    pieceElement.classList.add('king');
                }
                pieceElement.dataset.row = row;
                pieceElement.dataset.col = col;
                pieceElement.addEventListener('click', handlePieceClick);
                square.appendChild(pieceElement);
            }

            square.addEventListener('click', handleSquareClick);
            boardElement.appendChild(square);

            square.addEventListener('click', handleSquareClick);
            boardElement.appendChild(square);
        }
    }
}

// Handle piece click
function handlePieceClick(event) {
    event.stopPropagation(); // Prevent square click event
    const row = parseInt(event.target.dataset.row);
    const col = parseInt(event.target.dataset.col);
    const piece = board[row][col];

    if (piece && piece.color === currentPlayer) {
        selectedPiece = { row, col };
        renderBoard(); // Re-render to highlight selected piece and valid moves
        highlightValidMoves(row, col);
    }
}

// Handle square click
function handleSquareClick(event) {
    const row = parseInt(event.target.dataset.row);
    const col = parseInt(event.target.dataset.col);

    if (selectedPiece) {
        // Attempt to move the selected piece
        if (isValidMove(selectedPiece.row, selectedPiece.col, row, col)) {
            movePiece(selectedPiece.row, selectedPiece.col, row, col);
            // Check for kinging
            checkForKing(row, col);
            // Switch player
            switchPlayer();
            // Clear selection
            selectedPiece = null;
            renderBoard();
        } else {
            // Invalid move, clear selection
            selectedPiece = null;
            renderBoard();
        }
    }
}

// Highlight valid moves
function highlightValidMoves(row, col) {
    const piece = board[row][col];
    const possibleMoves = getPossibleMoves(row, col);

    possibleMoves.forEach(move => {
        const square = boardElement.querySelector(`[data-row="${move.row}"][data-col="${move.col}"]`);
        if (square) {
            square.classList.add('valid-move');
            // Add click listener to valid move squares
            square.addEventListener('click', handleSquareClick);
        }
    });
}

// Get possible moves for a piece
function getPossibleMoves(row, col) {
    const piece = board[row][col];
    const moves = [];
    const directions = piece.color === 'red' ? [[-1, -1], [-1, 1]] : [[1, -1], [1, 1]]; // Red moves up, Black moves down

    // If king, add reverse directions
    if (piece.isKing) {
        directions.push(...(piece.color === 'red' ? [[1, -1], [1, 1]] : [[-1, -1], [-1, 1]]));
    }

    for (const [dr, dc] of directions) {
        let newRow = row + dr;
        let newCol = col + dc;

        // Check if within board bounds
        if (newRow >= 0 && newRow < BOARD_SIZE && newCol >= 0 && newCol < BOARD_SIZE) {
            // Check if the target square is empty
            if (!board[newRow][newCol]) {
                moves.push({ row: newRow, col: newCol });
            } else {
                // Check for jumps (captures)
                const jumpedRow = newRow + dr;
                const jumpedCol = newCol + dc;
                const jumpedPiece = board[newRow][newCol];

                if (jumpedRow >= 0 && jumpedRow < BOARD_SIZE && jumpedCol >= 0 && jumpedCol < BOARD_SIZE && !board[jumpedRow][jumpedCol] && jumpedPiece.color !== piece.color) {
                    moves.push({ row: jumpedRow, col: jumpedCol, jumped: { row: newRow, col: newCol } });
                }
            }
        }
    }
    return moves;
}

// Check if a move is valid
function isValidMove(fromRow, fromCol, toRow, toCol) {
    const moves = getPossibleMoves(fromRow, fromCol);
    return moves.some(move => move.row === toRow && move.col === toCol);
}

// Move piece
function movePiece(fromRow, fromCol, toRow, toCol) {
    const piece = board[fromRow][fromCol];
    board[toRow][toCol] = piece;
    board[fromRow][fromCol] = null;

    // Handle jumps
    const move = getPossibleMoves(fromRow, fromCol).find(m => m.row === toRow && m.col === toCol);
    if (move && move.jumped) {
        board[move.jumped.row][move.jumped.col] = null;
    }
}

// Check for kinging
function checkForKing(row, col) {
    const piece = board[row][col];
    if (!piece.isKing) {
        if ((piece.color === 'red' && row === 0) || (piece.color === 'black' && row === BOARD_SIZE - 1)) {
            piece.isKing = true;
            renderBoard(); // Re-render to show king
        }
    }
}

// Switch player
function switchPlayer() {
    currentPlayer = currentPlayer === 'red' ? 'black' : 'red';
}

// Initialize game
initializeBoard();
renderBoard();
