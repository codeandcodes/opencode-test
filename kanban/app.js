// Kanban Board Application
(function() {
    'use strict';

    // Label colors configuration
    const LABEL_COLORS = [
        { name: 'red', class: 'label-red' },
        { name: 'orange', class: 'label-orange' },
        { name: 'yellow', class: 'label-yellow' },
        { name: 'green', class: 'label-green' },
        { name: 'blue', class: 'label-blue' },
        { name: 'purple', class: 'label-purple' },
        { name: 'pink', class: 'label-pink' },
        { name: 'cyan', class: 'label-cyan' }
    ];

    // Default columns configuration
    const DEFAULT_COLUMNS = [
        { id: 'backlog', title: 'Backlog', wipLimit: null },
        { id: 'in-progress', title: 'In Progress', wipLimit: 3 },
        { id: 'review', title: 'Review', wipLimit: 2 },
        { id: 'done', title: 'Done', wipLimit: null }
    ];

    // Demo cards data
    const DEMO_CARDS = [
        { id: 'card-1', title: 'Setup project structure', description: 'Initialize repository with proper folder structure and configuration files.', labels: ['blue', 'green'], dueDate: '2026-08-20', columnId: 'backlog' },
        { id: 'card-2', title: 'Design database schema', description: 'Create ERD and define all tables with relationships.', labels: ['purple'], dueDate: '2026-08-18', columnId: 'backlog' },
        { id: 'card-3', title: 'Implement authentication', description: 'Add user registration, login, and JWT token handling.', labels: ['red', 'blue'], dueDate: '2026-08-14', columnId: 'in-progress' },
        { id: 'card-4', title: 'Create API endpoints', description: 'Build RESTful API for CRUD operations.', labels: ['green'], dueDate: '2026-08-22', columnId: 'in-progress' },
        { id: 'card-5', title: 'Write unit tests', description: 'Cover all utility functions with comprehensive tests.', labels: ['yellow'], dueDate: '2026-08-25', columnId: 'in-progress' },
        { id: 'card-6', title: 'Setup CI/CD pipeline', description: 'Configure automated testing and deployment.', labels: ['orange'], dueDate: '2026-08-17', columnId: 'review' },
        { id: 'card-7', title: 'Performance optimization', description: 'Optimize database queries and add caching layer.', labels: ['red'], dueDate: '2026-08-12', columnId: 'review' },
        { id: 'card-8', title: 'Initial project setup', description: 'Completed the basic project scaffolding.', labels: ['green'], dueDate: '2026-08-10', columnId: 'done' },
        { id: 'card-9', title: 'Documentation draft', description: 'Write initial README and API documentation.', labels: ['blue'], dueDate: '2026-08-28', columnId: 'backlog' },
        { id: 'card-10', title: 'UI mockups', description: 'Create wireframes for main application screens.', labels: ['pink', 'purple'], dueDate: '2026-08-19', columnId: 'backlog' },
        { id: 'card-11', title: 'Security audit', description: 'Review code for security vulnerabilities.', labels: ['red', 'orange'], dueDate: '2026-08-30', columnId: 'backlog' },
        { id: 'card-12', title: 'Deploy to staging', description: 'Set up staging environment for testing.', labels: ['cyan'], dueDate: '2026-08-16', columnId: 'done' }
    ];

    // Application state
    let state = {
        columns: [],
        cards: [],
        columnOrder: [],
        history: [],
        historyIndex: -1,
        filters: {
            search: '',
            labels: []
        }
    };

    // Drag state
    let dragState = {
        draggingCard: null,
        draggingColumn: null,
        sourceColumnId: null,
        sourceIndex: null,
        targetColumnId: null,
        targetIndex: null
    };

    // DOM elements
    let elements = {};

    // Storage key
    const STORAGE_KEY = 'kanban-board-data';

    // ==================== State Management ====================

    function saveState() {
        localStorage.setItem(STORAGE_KEY, JSON.stringify({
            columns: state.columns,
            cards: state.cards,
            columnOrder: state.columnOrder
        }));
    }

    function loadState() {
        const stored = localStorage.getItem(STORAGE_KEY);
        if (stored) {
            try {
                const data = JSON.parse(stored);
                state.columns = data.columns || DEFAULT_COLUMNS;
                state.cards = data.cards || [];
                state.columnOrder = data.columnOrder || state.columns.map(c => c.id);
                return true;
            } catch (e) {
                console.error('Failed to load state:', e);
            }
        }
        return false;
    }

    function loadDemoData() {
        state.columns = JSON.parse(JSON.stringify(DEFAULT_COLUMNS));
        state.columnOrder = state.columns.map(c => c.id);
        state.cards = JSON.parse(JSON.stringify(DEMO_CARDS));
        saveState();
    }

    // ==================== History Management ====================

    function pushHistory() {
        // Remove any redo history
        state.history = state.history.slice(0, state.historyIndex + 1);
        
        // Save current state
        state.history.push({
            columns: JSON.parse(JSON.stringify(state.columns)),
            cards: JSON.parse(JSON.stringify(state.cards)),
            columnOrder: [...state.columnOrder]
        });

        // Limit history to 20 entries
        if (state.history.length > 20) {
            state.history.shift();
        } else {
            state.historyIndex++;
        }

        updateUndoButton();
    }

    function undo() {
        if (state.historyIndex <= 0) return;

        state.historyIndex--;
        const prevState = state.history[state.historyIndex];

        state.columns = JSON.parse(JSON.stringify(prevState.columns));
        state.cards = JSON.parse(JSON.stringify(prevState.cards));
        state.columnOrder = [...prevState.columnOrder];

        saveState();
        renderBoard();
        updateUndoButton();
    }

    function updateUndoButton() {
        const btn = elements.undoBtn;
        btn.disabled = state.historyIndex <= 0;
    }

    // ==================== Filtering ====================

    function getVisibleCards(columnId) {
        return state.cards.filter(card => {
            if (card.columnId !== columnId) return false;
            
            // Search filter
            if (state.filters.search) {
                const search = state.filters.search.toLowerCase();
                const matchesSearch = 
                    card.title.toLowerCase().includes(search) ||
                    card.description.toLowerCase().includes(search);
                if (!matchesSearch) return false;
            }

            // Label filter
            if (state.filters.labels.length > 0) {
                const hasAnyLabel = card.labels.some(label => 
                    state.filters.labels.includes(label)
                );
                if (!hasAnyLabel) return false;
            }

            return true;
        });
    }

    // ==================== Card Operations ====================

    function createCard(columnId) {
        pushHistory();

        const newCard = {
            id: 'card-' + Date.now(),
            title: 'New Card',
            description: '',
            labels: [],
            dueDate: '',
            columnId: columnId
        };

        state.cards.push(newCard);
        saveState();
        renderBoard();
        openCardModal(newCard.id);
    }

    function updateCard(cardId, updates) {
        pushHistory();

        const card = state.cards.find(c => c.id === cardId);
        if (card) {
            Object.assign(card, updates);
            saveState();
            renderBoard();
        }
    }

    function deleteCard(cardId) {
        pushHistory();

        state.cards = state.cards.filter(c => c.id !== cardId);
        saveState();
        renderBoard();
        closeCardModal();
    }

    function moveCard(cardId, targetColumnId, targetIndex) {
        pushHistory();

        const cardIndex = state.cards.findIndex(c => c.id === cardId);
        if (cardIndex === -1) return;

        const card = state.cards[cardIndex];
        const oldColumnId = card.columnId;

        // Remove from current position
        state.cards.splice(cardIndex, 1);

        // Adjust target index if moving within same column and index is after original position
        if (oldColumnId === targetColumnId && cardIndex < targetIndex) {
            targetIndex--;
        }

        // Insert at new position
        card.columnId = targetColumnId;
        state.cards.splice(targetIndex, 0, card);

        saveState();
        renderBoard();
    }

    // ==================== Column Operations ====================

    function getColumnCardCount(columnId) {
        return state.cards.filter(c => c.columnId === columnId).length;
    }

    function canAddToColumn(columnId) {
        const column = state.columns.find(c => c.id === columnId);
        if (!column || column.wipLimit === null) return true;
        return getColumnCardCount(columnId) < column.wipLimit;
    }

    function isColumnFull(columnId) {
        const column = state.columns.find(c => c.id === columnId);
        if (!column || column.wipLimit === null) return false;
        return getColumnCardCount(columnId) >= column.wipLimit;
    }

    function reorderColumns(fromIndex, toIndex) {
        pushHistory();

        const columnId = state.columnOrder[fromIndex];
        state.columnOrder.splice(fromIndex, 1);
        state.columnOrder.splice(toIndex, 0, columnId);

        saveState();
        renderBoard();
    }

    // ==================== Rendering ====================

    function renderBoard() {
        const board = elements.board;
        board.innerHTML = '';

        state.columnOrder.forEach((columnId, index) => {
            const column = state.columns.find(c => c.id === columnId);
            if (!column) return;

            const columnEl = createColumnElement(column, index);
            board.appendChild(columnEl);
        });

        renderLabelFilters();
    }

    function createColumnElement(column, index) {
        const columnEl = document.createElement('div');
        columnEl.className = 'column';
        columnEl.dataset.columnId = column.id;

        const count = getColumnCardCount(column.id);
        const wipClass = column.wipLimit !== null && count >= column.wipLimit ? 'exceeded' : '';
        const wipText = column.wipLimit !== null ? `${count}/${column.wipLimit}` : count.toString();

        columnEl.innerHTML = `
            <div class="column-header" data-column-id="${column.id}">
                <span class="column-title">${column.title}</span>
                <span class="column-wip ${wipClass}">${wipText}</span>
            </div>
            <div class="column-body" data-column-id="${column.id}"></div>
            <button class="add-card-btn">+ Add Card</button>
        `;

        const header = columnEl.querySelector('.column-header');
        const body = columnEl.querySelector('.column-body');
        const addBtn = columnEl.querySelector('.add-card-btn');

        // Column header drag (for reordering columns)
        setupColumnHeaderDrag(header, index);

        // Column body drag events
        setupColumnBodyDrag(body, column.id);

        // Add card button
        addBtn.addEventListener('click', () => createCard(column.id));

        // Render cards
        const visibleCards = getVisibleCards(column.id);
        visibleCards.forEach((card, cardIndex) => {
            const cardEl = createCardElement(card);
            body.appendChild(cardEl);
        });

        return columnEl;
    }

    function createCardElement(card) {
        const cardEl = document.createElement('div');
        cardEl.className = 'card';
        cardEl.dataset.cardId = card.id;

        const labelsHtml = card.labels.map(label => {
            const color = LABEL_COLORS.find(l => l.name === label);
            return `<span class="card-label ${color ? color.class : 'label-blue'}">${label}</span>`;
        }).join('');

        const dueDateHtml = card.dueDate ? `
            <span class="card-due-date ${isOverdue(card.dueDate) ? 'overdue' : ''}">
                📅 ${formatDate(card.dueDate)}
            </span>
        ` : '';

        cardEl.innerHTML = `
            <div class="card-top">
                <span class="card-title">${escapeHtml(card.title)}</span>
            </div>
            ${labelsHtml ? `<div class="card-labels">${labelsHtml}</div>` : ''}
            ${card.description ? `<div class="card-description">${escapeHtml(card.description)}</div>` : ''}
            <div class="card-footer">
                ${dueDateHtml}
            </div>
        `;

        // Card click to edit
        cardEl.addEventListener('click', (e) => {
            if (!e.target.closest('.card-label')) {
                openCardModal(card.id);
            }
        });

        // Card drag events
        setupCardDrag(cardEl, card.id);

        return cardEl;
    }

    function renderLabelFilters() {
        const container = elements.labelFilters;
        container.innerHTML = '';

        LABEL_COLORS.forEach(label => {
            const btn = document.createElement('button');
            btn.className = 'label-filter-btn';
            btn.dataset.label = label.name;
            
            const span = document.createElement('span');
            span.className = `card-label ${label.class}`;
            span.textContent = label.name;
            btn.appendChild(span);

            if (state.filters.labels.includes(label.name)) {
                btn.classList.add('active');
            }

            btn.addEventListener('click', () => {
                const index = state.filters.labels.indexOf(label.name);
                if (index > -1) {
                    state.filters.labels.splice(index, 1);
                } else {
                    state.filters.labels.push(label.name);
                }
                renderBoard();
            });

            container.appendChild(btn);
        });
    }

    // ==================== Drag and Drop - Cards ====================

    function setupCardDrag(cardEl, cardId) {
        cardEl.addEventListener('dragstart', (e) => {
            dragState.draggingCard = cardId;
            const card = state.cards.find(c => c.id === cardId);
            dragState.sourceColumnId = card.columnId;
            
            const cardIndex = state.cards.findIndex(c => c.id === cardId);
            dragState.sourceIndex = cardIndex;

            cardEl.classList.add('dragging');
            e.dataTransfer.effectAllowed = 'move';
            e.dataTransfer.setData('text/plain', cardId);
        });

        cardEl.addEventListener('dragend', () => {
            cardEl.classList.remove('dragging');
            clearAllIndicators();
            
            dragState.draggingCard = null;
            dragState.sourceColumnId = null;
            dragState.sourceIndex = null;
            dragState.targetColumnId = null;
            dragState.targetIndex = null;
        });
    }

    function setupColumnBodyDrag(body, columnId) {
        body.addEventListener('dragover', (e) => {
            e.preventDefault();
            e.dataTransfer.dropEffect = 'move';

            if (!dragState.draggingCard) return;

            const column = state.columns.find(c => c.id === columnId);
            const currentCount = getVisibleCards(columnId).length;
            const canAccept = column.wipLimit === null || currentCount < column.wipLimit;

            // Check if we can drop here
            if (!canAccept && dragState.sourceColumnId !== columnId) {
                body.classList.add('drag-over-full');
                body.classList.remove('drag-over');
                return;
            }

            body.classList.add('drag-over');
            body.classList.remove('drag-over-full');

            // Find insertion position
            const cardElements = Array.from(body.querySelectorAll('.card:not(.dragging)'));
            let insertBefore = null;
            let insertIndex = 0;

            const rect = body.getBoundingClientRect();
            const mouseY = e.clientY - rect.top;

            if (cardElements.length === 0) {
                // Empty column - insert at end
                insertIndex = state.cards.filter(c => c.columnId === columnId).length;
            } else {
                // Find closest card
                let minDistance = Infinity;
                cardElements.forEach((cardEl, index) => {
                    const cardRect = cardEl.getBoundingClientRect();
                    const cardMiddle = cardRect.top + cardRect.height / 2;
                    const distance = Math.abs(mouseY - (cardRect.top - rect.top));

                    if (distance < minDistance) {
                        minDistance = distance;
                        insertBefore = cardEl;
                        insertIndex = index;
                    }
                });
            }

            clearAllIndicators();
            showInsertionIndicator(body, insertBefore, insertIndex);
            
            dragState.targetColumnId = columnId;
            dragState.targetIndex = insertIndex;
        });

        body.addEventListener('dragleave', (e) => {
            if (!body.contains(e.relatedTarget)) {
                body.classList.remove('drag-over', 'drag-over-full');
                clearAllIndicators();
            }
        });

        body.addEventListener('drop', (e) => {
            e.preventDefault();
            body.classList.remove('drag-over', 'drag-over-full');
            clearAllIndicators();

            if (!dragState.draggingCard) return;

            const column = state.columns.find(c => c.id === columnId);
            const visibleCount = getVisibleCards(columnId).length;
            const canAccept = column.wipLimit === null || visibleCount < column.wipLimit;

            // Reject drop if column is full and not the source column
            if (!canAccept && dragState.sourceColumnId !== columnId) {
                body.classList.add('drag-over-full');
                const wipEl = body.closest('.column').querySelector('.column-wip');
                if (wipEl) {
                    wipEl.classList.add('rejected');
                    setTimeout(() => wipEl.classList.remove('rejected'), 300);
                }
                return;
            }

            // Perform the move
            moveCard(dragState.draggingCard, columnId, dragState.targetIndex);
        });
    }

    function showInsertionIndicator(container, beforeElement, index) {
        const indicator = document.createElement('div');
        indicator.className = 'insertion-indicator';

        if (!beforeElement) {
            // Insert at end
            indicator.classList.add('bottom');
            container.appendChild(indicator);
        } else {
            // Insert before element
            beforeElement.parentNode.insertBefore(indicator, beforeElement);
        }
    }

    function clearAllIndicators() {
        document.querySelectorAll('.insertion-indicator').forEach(el => el.remove());
        document.querySelectorAll('.drag-over, .drag-over-full').forEach(el => {
            el.classList.remove('drag-over', 'drag-over-full');
        });
    }

    // ==================== Drag and Drop - Columns ====================

    function setupColumnHeaderDrag(header, index) {
        header.draggable = true;

        header.addEventListener('dragstart', (e) => {
            dragState.draggingColumn = index;
            e.dataTransfer.effectAllowed = 'move';
            e.dataTransfer.setData('text/plain', index.toString());
            header.style.opacity = '0.5';
        });

        header.addEventListener('dragend', () => {
            header.style.opacity = '1';
            dragState.draggingColumn = null;
        });

        header.addEventListener('dragover', (e) => {
            e.preventDefault();
            if (dragState.draggingColumn === null || dragState.draggingColumn === index) return;

            const columnEl = header.closest('.column');
            const columns = Array.from(document.querySelectorAll('.column'));
            const targetIndex = columns.indexOf(columnEl);

            // Visual feedback for column reordering
            columns.forEach((col, i) => {
                col.style.transform = i === targetIndex && i !== dragState.draggingColumn 
                    ? 'scale(1.02)' 
                    : 'none';
            });
        });

        header.addEventListener('dragleave', () => {
            document.querySelectorAll('.column').forEach(col => {
                col.style.transform = 'none';
            });
        });

        header.addEventListener('drop', (e) => {
            e.preventDefault();
            document.querySelectorAll('.column').forEach(col => {
                col.style.transform = 'none';
            });

            if (dragState.draggingColumn === null || dragState.draggingColumn === index) return;

            reorderColumns(dragState.draggingColumn, index);
        });
    }

    // ==================== Modal / Card Editing ====================

    function openCardModal(cardId) {
        const card = state.cards.find(c => c.id === cardId);
        if (!card) return;

        elements.cardId.value = card.id;
        elements.cardColumnId.value = card.columnId;
        elements.cardTitle.value = card.title;
        elements.cardDescription.value = card.description;
        elements.cardDueDate.value = card.dueDate || '';

        // Render label checkboxes
        const checkboxes = elements.cardLabelCheckboxes;
        checkboxes.innerHTML = '';
        LABEL_COLORS.forEach(label => {
            const div = document.createElement('label');
            div.className = 'label-checkbox';
            const checked = card.labels.includes(label.name) ? 'checked' : '';
            div.innerHTML = `
                <input type="checkbox" value="${label.name}" ${checked}>
                <span class="${label.class}">${label.name}</span>
            `;
            checkboxes.appendChild(div);
        });

        // Setup delete button
        elements.modalDelete.onclick = () => {
            if (confirm('Are you sure you want to delete this card?')) {
                deleteCard(cardId);
            }
        };

        elements.cardModal.classList.add('active');
    }

    function closeCardModal() {
        elements.cardModal.classList.remove('active');
    }

    function setupCardForm() {
        elements.cardForm.onsubmit = (e) => {
            e.preventDefault();

            const cardId = elements.cardId.value;
            const title = elements.cardTitle.value.trim();
            const description = elements.cardDescription.value.trim();
            const dueDate = elements.cardDueDate.value;

            // Get selected labels
            const labels = [];
            elements.cardLabelCheckboxes.querySelectorAll('input:checked').forEach(input => {
                labels.push(input.value);
            });

            if (title) {
                updateCard(cardId, { title, description, labels, dueDate });
                closeCardModal();
            }
        };
    }

    // ==================== Search ====================

    function setupSearch() {
        elements.searchInput.addEventListener('input', (e) => {
            state.filters.search = e.target.value;
            renderBoard();
        });
    }

    // ==================== Keyboard Shortcuts ====================

    function setupKeyboardShortcuts() {
        document.addEventListener('keydown', (e) => {
            if ((e.ctrlKey || e.metaKey) && e.key === 'z') {
                e.preventDefault();
                undo();
            }
        });
    }

    // ==================== Utility Functions ====================

    function escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }

    function isOverdue(dateStr) {
        if (!dateStr) return false;
        const dueDate = new Date(dateStr);
        const today = new Date();
        today.setHours(0, 0, 0, 0);
        return dueDate < today;
    }

    function formatDate(dateStr) {
        if (!dateStr) return '';
        const date = new Date(dateStr);
        return date.toLocaleDateString('en-US', { month: 'short', day: 'numeric' });
    }

    // ==================== Initialization ====================

    function init() {
        // Cache DOM elements
        elements = {
            board: document.getElementById('board'),
            searchInput: document.getElementById('searchInput'),
            labelFilters: document.getElementById('labelFilters'),
            undoBtn: document.getElementById('undoBtn'),
            cardModal: document.getElementById('cardModal'),
            modalClose: document.getElementById('modalClose'),
            cardForm: document.getElementById('cardForm'),
            cardId: document.getElementById('cardId'),
            cardColumnId: document.getElementById('cardColumnId'),
            cardTitle: document.getElementById('cardTitle'),
            cardDescription: document.getElementById('cardDescription'),
            cardDueDate: document.getElementById('cardDueDate'),
            cardLabelCheckboxes: document.getElementById('cardLabelCheckboxes'),
            modalDelete: document.getElementById('modalDelete')
        };

        // Load or initialize data
        if (!loadState()) {
            loadDemoData();
        }

        // Setup event listeners
        elements.undoBtn.addEventListener('click', undo);
        elements.modalClose.addEventListener('click', closeCardModal);
        elements.cardModal.addEventListener('click', (e) => {
            if (e.target === elements.cardModal) closeCardModal();
        });
        setupCardForm();
        setupSearch();
        setupKeyboardShortcuts();

        // Initial render
        renderBoard();
        updateUndoButton();
    }

    // Start the app when DOM is ready
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', init);
    } else {
        init();
    }
})();
