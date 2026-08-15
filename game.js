// Checkers Game - Complete Implementation

class CheckersGame {
    constructor() {
        this.board = [];
        this.currentPlayer = 'red';
        this.selectedPiece = null;
        this.legalMoves = [];
        this.moveHistory = [];
        this.stateHistory = [];
        this.lastMove = null;
        this.gameOver = false;
        this.aiEnabled = false;
        this.aiDepth = 3;
        this.mustCaptureFrom = null;
        
        this.initBoard();
        this.renderBoard();
        this.setupEventListeners();
        this.updateTurnIndicator();
    }

    initBoard() {
        this.board = [];
        for (let row = 0; row < 8; row++) {
            this.board[row] = [];
            for (let col = 0; col < 8; col++) {
                if ((row + col) % 2 === 1) {
                    if (row < 3) {
                        this.board[row][col] = { color: 'black', isKing: false };
                    } else if (row > 4) {
                        this.board[row][col] = { color: 'red', isKing: false };
                    } else {
                        this.board[row][col] = null;
                    }
                } else {
                    this.board[row][col] = 'light';
                }
            }
        }
    }

    renderBoard() {
        const boardEl = document.getElementById('board');
        boardEl.innerHTML = '';

        for (let row = 0; row < 8; row++) {
            for (let col = 0; col < 8; col++) {
                const square = document.createElement('div');
                square.className = 'square';
                square.dataset.row = row;
                square.dataset.col = col;

                const isDark = (row + col) % 2 === 1;
                square.classList.add(isDark ? 'dark' : 'light');

                // Highlight last move
                if (this.lastMove) {
                    if ((this.lastMove.fromRow === row && this.lastMove.fromCol === col) ||
                        (this.lastMove.toRow === row && this.lastMove.toCol === col)) {
                        square.classList.add('last-move');
                    }
                }

                // Highlight selected piece
                if (this.selectedPiece && this.selectedPiece.row === row && this.selectedPiece.col === col) {
                    square.classList.add('selected');
                }

                // Highlight legal destinations
                if (this.legalMoves.some(m => m.toRow === row && m.toCol === col)) {
                    square.classList.add('highlight');
                    square.addEventListener('click', () => this.handleSquareClick(row, col));
                }

                const piece = this.board[row][col];
                if (piece && piece !== 'light') {
                    const pieceEl = document.createElement('div');
                    pieceEl.className = `piece ${piece.color}`;
                    if (piece.isKing) {
                        pieceEl.classList.add('king');
                    }
                    square.appendChild(pieceEl);
                }

                if (isDark) {
                    square.addEventListener('click', () => this.handleSquareClick(row, col));
                }

                boardEl.appendChild(square);
            }
        }
    }

    handleSquareClick(row, col) {
        if (this.gameOver) return;
        if (this.aiEnabled && this.currentPlayer === 'black') return;

        const piece = this.board[row][col];

        // If we're in a multi-jump sequence, only allow clicking the jumping piece
        if (this.mustCaptureFrom) {
            if (row === this.mustCaptureFrom.row && col === this.mustCaptureFrom.col) {
                return; // Already selected
            }
            // Check if this is a legal capture move
            const move = this.legalMoves.find(m => m.toRow === row && m.toCol === col);
            if (move) {
                this.makeMove(move);
            }
            return;
        }

        // Select a piece
        if (piece && piece !== 'light' && piece.color === this.currentPlayer) {
            this.selectedPiece = { row, col };
            this.legalMoves = this.getLegalMovesForPiece(row, col);
            this.renderBoard();
            return;
        }

        // Move to a highlighted square
        if (this.selectedPiece) {
            const move = this.legalMoves.find(m => m.toRow === row && m.toCol === col);
            if (move) {
                this.makeMove(move);
            }
        }
    }

    getLegalMovesForPiece(row, col) {
        const piece = this.board[row][col];
        if (!piece || piece === 'light') return [];

        const allMoves = this.getAllLegalMoves(this.currentPlayer);
        return allMoves.filter(m => m.fromRow === row && m.fromCol === col);
    }

    getAllLegalMoves(player) {
        const moves = [];
        const captures = [];

        for (let row = 0; row < 8; row++) {
            for (let col = 0; col < 8; col++) {
                const piece = this.board[row][col];
                if (piece && piece !== 'light' && piece.color === player) {
                    const pieceMoves = this.getMovesForPiece(row, col, piece);
                    pieceMoves.forEach(m => {
                        if (m.isCapture) {
                            captures.push(m);
                        } else {
                            moves.push(m);
                        }
                    });
                }
            }
        }

        // Forced capture: if captures exist, only return capture moves
        return captures.length > 0 ? captures : moves;
    }

    getMovesForPiece(row, col, piece) {
        const moves = [];
        const directions = [];

        if (piece.color === 'red' || piece.isKing) {
            directions.push({ dr: -1, dc: -1 }, { dr: -1, dc: 1 });
        }
        if (piece.color === 'black' || piece.isKing) {
            directions.push({ dr: 1, dc: -1 }, { dr: 1, dc: 1 });
        }

        for (const dir of directions) {
            const newRow = row + dir.dr;
            const newCol = col + dir.dc;

            if (this.isValidPosition(newRow, newCol)) {
                // Regular move
                if (this.board[newRow][newCol] === null) {
                    if (!this.mustCaptureFrom) {
                        moves.push({
                            fromRow: row,
                            fromCol: col,
                            toRow: newRow,
                            toCol: newCol,
                            isCapture: false
                        });
                    }
                }
                // Capture move
                else if (this.board[newRow][newCol] !== 'light' && 
                         this.board[newRow][newCol].color !== piece.color) {
                    const jumpRow = newRow + dir.dr;
                    const jumpCol = newCol + dir.dc;
                    if (this.isValidPosition(jumpRow, jumpCol) && this.board[jumpRow][jumpCol] === null) {
                        moves.push({
                            fromRow: row,
                            fromCol: col,
                            toRow: jumpRow,
                            toCol: jumpCol,
                            isCapture: true,
                            capturedRow: newRow,
                            capturedCol: newCol
                        });
                    }
                }
            }
        }

        return moves;
    }

    isValidPosition(row, col) {
        return row >= 0 && row < 8 && col >= 0 && col < 8;
    }

    makeMove(move) {
        // Save state for undo
        this.saveState();

        const piece = this.board[move.fromRow][move.fromCol];
        this.board[move.fromRow][move.fromCol] = null;
        this.board[move.toRow][move.toCol] = piece;

        // Record captured piece for history
        let capturedNotation = '';
        if (move.isCapture) {
            this.board[move.capturedRow][move.capturedCol] = null;
            capturedNotation = `x${this.getNotation(move.capturedRow, move.capturedCol)}`;
        }

        // Check for king promotion
        let promoted = false;
        if (!piece.isKing) {
            if ((piece.color === 'red' && move.toRow === 0) ||
                (piece.color === 'black' && move.toRow === 7)) {
                piece.isKing = true;
                promoted = true;
            }
        }

        // Handle multi-jump
        let multiJumpContinues = false;
        if (move.isCapture) {
            const followUpMoves = this.getMovesForPiece(move.toRow, move.toCol, piece);
            const captureMoves = followUpMoves.filter(m => m.isCapture);
            if (captureMoves.length > 0) {
                multiJumpContinues = true;
                this.mustCaptureFrom = { row: move.toRow, col: move.toCol };
                this.legalMoves = captureMoves;
                this.lastMove = move;
                this.renderBoard();
                
                // If AI is playing and it's a multi-jump, continue immediately
                if (this.aiEnabled && this.currentPlayer === 'black') {
                    setTimeout(() => this.aiMakeMove(), 300);
                }
                return;
            }
        }

        // End turn
        this.mustCaptureFrom = null;
        this.selectedPiece = null;
        this.legalMoves = [];
        this.lastMove = move;

        // Add to history
        const notation = this.getMoveNotation(move, piece, capturedNotation, promoted);
        this.moveHistory.push({
            player: this.currentPlayer,
            notation: notation,
            moveNumber: Math.ceil(this.moveHistory.length / 2)
        });
        this.updateHistoryDisplay();

        // Switch player
        this.currentPlayer = this.currentPlayer === 'red' ? 'black' : 'red';
        this.updateTurnIndicator();

        // Check game over
        if (this.checkGameOver()) {
            this.renderBoard();
            return;
        }

        this.renderBoard();

        // AI turn
        if (this.aiEnabled && this.currentPlayer === 'black' && !this.gameOver) {
            setTimeout(() => this.aiMakeMove(), 500);
        }
    }

    getNotation(row, col) {
        const files = ['a', 'b', 'c', 'd', 'e', 'f', 'g', 'h'];
        const ranks = ['8', '7', '6', '5', '4', '3', '2', '1'];
        return files[col] + ranks[row];
    }

    getMoveNotation(move, piece, capturedNotation, promoted) {
        const fromNotation = this.getNotation(move.fromRow, move.fromCol);
        const toNotation = this.getNotation(move.toRow, move.toCol);
        const pieceNotation = piece.isKing ? 'K' : (piece.color === 'red' ? 'R' : 'B');
        let notation = `${pieceNotation}${fromNotation}${capturedNotation}${toNotation}`;
        if (promoted) {
            notation += '=';
        }
        return notation;
    }

    saveState() {
        const state = {
            board: this.board.map(row => row.map(cell => 
                cell === 'light' ? 'light' : 
                cell === null ? null : 
                { color: cell.color, isKing: cell.isKing }
            )),
            currentPlayer: this.currentPlayer,
            selectedPiece: this.selectedPiece ? { ...this.selectedPiece } : null,
            legalMoves: this.legalMoves.map(m => ({ ...m })),
            moveHistory: this.moveHistory.map(h => ({ ...h })),
            lastMove: this.lastMove ? { ...this.lastMove } : null,
            mustCaptureFrom: this.mustCaptureFrom ? { ...this.mustCaptureFrom } : null,
            gameOver: this.gameOver
        };
        this.stateHistory.push(state);
    }

    undo() {
        if (this.stateHistory.length === 0 || this.gameOver) return;

        // If AI is enabled, undo two moves (player + AI)
        let statesToUndo = 1;
        if (this.aiEnabled && this.stateHistory.length >= 2) {
            statesToUndo = 2;
        }

        for (let i = 0; i < statesToUndo && this.stateHistory.length > 0; i++) {
            const state = this.stateHistory.pop();
            this.board = state.board;
            this.currentPlayer = state.currentPlayer;
            this.selectedPiece = state.selectedPiece;
            this.legalMoves = state.legalMoves;
            this.moveHistory = state.moveHistory;
            this.lastMove = state.lastMove;
            this.mustCaptureFrom = state.mustCaptureFrom;
            this.gameOver = state.gameOver;
        }

        this.moveHistory = this.moveHistory.filter(h => 
            h.player === this.currentPlayer || 
            (this.aiEnabled && this.stateHistory.length % 2 === 0)
        );
        
        // Recalculate move numbers
        let redCount = 0;
        let blackCount = 0;
        this.moveHistory = this.moveHistory.filter(h => {
            if (h.player === 'red') {
                redCount++;
                h.moveNumber = redCount;
                return true;
            } else {
                blackCount++;
                h.moveNumber = blackCount;
                return true;
            }
        });

        this.selectedPiece = null;
        this.legalMoves = [];
        this.updateTurnIndicator();
        this.updateHistoryDisplay();
        this.renderBoard();
    }

    checkGameOver() {
        const moves = this.getAllLegalMoves(this.currentPlayer);
        
        // Count pieces
        let redCount = 0;
        let blackCount = 0;
        for (let row = 0; row < 8; row++) {
            for (let col = 0; col < 8; col++) {
                const piece = this.board[row][col];
                if (piece && piece !== 'light') {
                    if (piece.color === 'red') redCount++;
                    else blackCount++;
                }
            }
        }

        let status = '';
        
        if (moves.length === 0) {
            this.gameOver = true;
            if (this.currentPlayer === 'red') {
                status = 'Black wins!';
            } else {
                status = 'Red wins!';
            }
        } else if (redCount === 0) {
            this.gameOver = true;
            status = 'Black wins!';
        } else if (blackCount === 0) {
            this.gameOver = true;
            status = 'Red wins!';
        }

        const statusEl = document.getElementById('game-status');
        statusEl.textContent = status;
        return this.gameOver;
    }

    updateTurnIndicator() {
        const indicator = document.getElementById('turn-indicator');
        indicator.textContent = `Turn: ${this.currentPlayer.charAt(0).toUpperCase() + this.currentPlayer.slice(1)}`;
        indicator.className = this.currentPlayer;
    }

    updateHistoryDisplay() {
        const historyEl = document.getElementById('move-history');
        historyEl.innerHTML = '';

        for (let i = 0; i < this.moveHistory.length; i += 2) {
            const moveEntry = document.createElement('div');
            moveEntry.className = 'move-entry';
            
            const redMove = this.moveHistory[i];
            const blackMove = this.moveHistory[i + 1];
            
            let html = `<span class="move-number">${redMove.moveNumber}.</span>`;
            html += `<span class="move-text">${redMove.notation}</span>`;
            if (blackMove) {
                html += ` <span class="move-text">${blackMove.notation}</span>`;
            }
            
            moveEntry.innerHTML = html;
            historyEl.appendChild(moveEntry);
        }

        historyEl.scrollTop = historyEl.scrollHeight;
    }

    reset() {
        this.board = [];
        this.currentPlayer = 'red';
        this.selectedPiece = null;
        this.legalMoves = [];
        this.moveHistory = [];
        this.stateHistory = [];
        this.lastMove = null;
        this.gameOver = false;
        this.mustCaptureFrom = null;
        
        this.initBoard();
        this.renderBoard();
        this.updateTurnIndicator();
        this.updateHistoryDisplay();
        document.getElementById('game-status').textContent = '';
    }

    // AI Implementation using Minimax with Alpha-Beta Pruning
    aiMakeMove() {
        if (this.gameOver) return;

        const depth = this.aiDepth;
        const bestMove = this.minimax(depth, true, -Infinity, Infinity);
        
        if (bestMove.move) {
            this.makeMove(bestMove.move);
        }
    }

    minimax(depth, isMaximizing, alpha, beta) {
        if (depth === 0) {
            return { score: this.evaluateBoard() };
        }

        const player = isMaximizing ? 'black' : 'red';
        const moves = this.getAllLegalMoves(player);

        if (moves.length === 0) {
            return { score: isMaximizing ? -1000 : 1000 };
        }

        let bestMove = null;

        if (isMaximizing) {
            let maxScore = -Infinity;
            for (const move of moves) {
                const savedState = this.cloneBoard();
                this.simulateMove(move);
                const result = this.minimax(depth - 1, false, alpha, beta);
                this.restoreBoard(savedState);

                if (result.score > maxScore) {
                    maxScore = result.score;
                    bestMove = move;
                }
                alpha = Math.max(alpha, result.score);
                if (beta <= alpha) break;
            }
            return { score: maxScore, move: bestMove };
        } else {
            let minScore = Infinity;
            for (const move of moves) {
                const savedState = this.cloneBoard();
                this.simulateMove(move);
                const result = this.minimax(depth - 1, true, alpha, beta);
                this.restoreBoard(savedState);

                if (result.score < minScore) {
                    minScore = result.score;
                    bestMove = move;
                }
                beta = Math.min(beta, result.score);
                if (beta <= alpha) break;
            }
            return { score: minScore, move: bestMove };
        }
    }

    cloneBoard() {
        return {
            board: this.board.map(row => row.map(cell => 
                cell === 'light' ? 'light' : 
                cell === null ? null : 
                { color: cell.color, isKing: cell.isKing }
            )),
            mustCaptureFrom: this.mustCaptureFrom ? { ...this.mustCaptureFrom } : null
        };
    }

    restoreBoard(savedState) {
        this.board = savedState.board;
        this.mustCaptureFrom = savedState.mustCaptureFrom;
    }

    simulateMove(move) {
        const piece = this.board[move.fromRow][move.fromCol];
        this.board[move.fromRow][move.fromCol] = null;
        this.board[move.toRow][move.toCol] = piece;

        if (move.isCapture) {
            this.board[move.capturedRow][move.capturedCol] = null;
        }

        if (!piece.isKing) {
            if ((piece.color === 'red' && move.toRow === 0) ||
                (piece.color === 'black' && move.toRow === 7)) {
                piece.isKing = true;
            }
        }

        // Handle multi-jump for simulation
        if (move.isCapture) {
            const followUpMoves = this.getMovesForPiece(move.toRow, move.toCol, piece);
            const captureMoves = followUpMoves.filter(m => m.isCapture);
            if (captureMoves.length > 0) {
                this.mustCaptureFrom = { row: move.toRow, col: move.toCol };
                // Continue the multi-jump in simulation
                const nextMove = captureMoves[0];
                this.simulateMove(nextMove);
                return;
            }
        }
        this.mustCaptureFrom = null;
    }

    evaluateBoard() {
        let score = 0;

        for (let row = 0; row < 8; row++) {
            for (let col = 0; col < 8; col++) {
                const piece = this.board[row][col];
                if (piece && piece !== 'light') {
                    let value = 10;
                    
                    // King bonus
                    if (piece.isKing) {
                        value = 20;
                    }

                    // Position bonus (control center)
                    if (col >= 2 && col <= 5 && row >= 2 && row <= 5) {
                        value += 2;
                    }

                    // Advancement bonus
                    if (piece.color === 'black') {
                        value += row;
                    } else {
                        value += (7 - row);
                    }

                    // Back row defense bonus
                    if (piece.color === 'black' && row === 7) {
                        value += 3;
                    } else if (piece.color === 'red' && row === 0) {
                        value += 3;
                    }

                    if (piece.color === 'black') {
                        score += value;
                    } else {
                        score -= value;
                    }
                }
            }
        }

        return score;
    }

    setupEventListeners() {
        document.getElementById('undo-btn').addEventListener('click', () => this.undo());
        document.getElementById('reset-btn').addEventListener('click', () => this.reset());
        
        const aiCheckbox = document.getElementById('ai-enabled');
        aiCheckbox.addEventListener('change', (e) => {
            this.aiEnabled = e.target.checked;
        });

        const aiDepthInput = document.getElementById('ai-depth');
        aiDepthInput.addEventListener('change', (e) => {
            this.aiDepth = Math.max(1, Math.min(5, parseInt(e.target.value) || 3));
        });
    }
}

// Initialize game when DOM is loaded
document.addEventListener('DOMContentLoaded', () => {
    new CheckersGame();
});
