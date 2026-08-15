// Calendar App - Self-contained static web app
// DST Guarantee: All recurrence expansions use calendar components (year/month/day + hour)
// to build occurrence dates, never by adding 7x24h to a timestamp. This ensures that
// events recurring at a specific local time (e.g., 09:00) remain at that local time
// across DST transitions.

(function() {
  'use strict';

  // Storage key
  const STORAGE_KEY = 'calendar_events_v1';

  // State
  let state = {
    currentDate: new Date(),
    view: 'month', // 'month' or 'week'
    events: [],
    editingEventId: null,
    editingEventSeriesId: null,
    deleteMode: null, // 'this' or 'all' when deleting recurring event
    dragState: null,
    dragEvent: null
  };

  // Demo data
  function loadDemoData() {
    const baseDate = new Date();
    const year = baseDate.getFullYear();
    const month = baseDate.getMonth();
    const day = baseDate.getDate();

    // Weekly stand-up (Mon/Wed/Fri at 09:00-09:30)
    const standUp = {
      id: generateId(),
      seriesId: generateId(),
      title: 'Team Stand-up',
      start: createDateTime(year, month, day, 9, 0),
      end: createDateTime(year, month, day, 9, 30),
      color: '#3498db',
      recurrence: {
        type: 'weekly',
        days: [1, 3, 5] // Mon, Wed, Fri
      },
      exceptions: {}
    };

    // Monthly review (2nd Tuesday at 14:00-15:00)
    const secondTuesday = getNthWeekdayOfMonth(year, month, 2, 2); // 2nd Tuesday
    const monthlyReview = {
      id: generateId(),
      seriesId: generateId(),
      title: 'Monthly Review',
      start: createDateTime(year, month, secondTuesday, 14, 0),
      end: createDateTime(year, month, secondTuesday, 15, 0),
      color: '#9b59b6',
      recurrence: {
        type: 'monthly',
        occurrence: 2,
        dayOfWeek: 2 // Tuesday
      },
      exceptions: {}
    };

    // An event with a moved occurrence exception
    const weeklyMeeting = {
      id: generateId(),
      seriesId: generateId(),
      title: 'Weekly Sync',
      start: createDateTime(year, month, 15, 10, 0),
      end: createDateTime(year, month, 15, 11, 0),
      color: '#27ae60',
      recurrence: {
        type: 'weekly',
        days: [2] // Tuesday
      },
      exceptions: {}
    };

    // Add a moved occurrence exception (move one occurrence by 1 hour)
    const exceptionDate = new Date(createDateTime(year, month, 15, 10, 0));
    exceptionDate.setDate(exceptionDate.getDate() + 10); // 10 days later
    const exceptionKey = formatDateKey(exceptionDate);
    weeklyMeeting.exceptions[exceptionKey] = {
      type: 'moved',
      start: createDateTime(exceptionDate.getFullYear(), exceptionDate.getMonth(), exceptionDate.getDate(), 11, 0),
      end: createDateTime(exceptionDate.getFullYear(), exceptionDate.getMonth(), exceptionDate.getDate(), 12, 0)
    };

    state.events = [standUp, monthlyReview, weeklyMeeting];
    saveEvents();
  }

  // Utility functions
  function generateId() {
    return 'evt_' + Date.now() + '_' + Math.random().toString(36).substr(2, 9);
  }

  function createDateTime(year, month, day, hour, minute) {
    // Use calendar components directly - DST safe
    return new Date(year, month, day, hour, minute, 0, 0);
  }

  function formatDateKey(date) {
    return date.getFullYear() + '-' + String(date.getMonth() + 1).padStart(2, '0') + '-' + String(date.getDate()).padStart(2, '0');
  }

  function formatDateTime(date) {
    const year = date.getFullYear();
    const month = String(date.getMonth() + 1).padStart(2, '0');
    const day = String(date.getDate()).padStart(2, '0');
    const hour = String(date.getHours()).padStart(2, '0');
    const minute = String(date.getMinutes()).padStart(2, '0');
    return `${year}-${month}-${day}T${hour}:${minute}`;
  }

  function getNthWeekdayOfMonth(year, month, n, dayOfWeek) {
    let date = new Date(year, month, 1);
    let count = 0;
    while (date.getMonth() === month) {
      if (date.getDay() === dayOfWeek) {
        count++;
        if (count === n) {
          return date.getDate();
        }
      }
      date.setDate(date.getDate() + 1);
    }
    return null; // No such occurrence (e.g., 5th weekday in a month with only 4)
  }

  function getLastWeekdayOfMonth(year, month, dayOfWeek) {
    let date = new Date(year, month + 1, 0); // Last day of month
    while (date.getMonth() === month) {
      if (date.getDay() === dayOfWeek) {
        return date.getDate();
      }
      date.setDate(date.getDate() - 1);
    }
    return null;
  }

  // DST-Safe recurrence expansion
  // Builds occurrences from calendar components, never by adding 7x24h to timestamp
  function expandRecurrence(event, startDate, endDate) {
    const occurrences = [];
    const start = new Date(event.start);
    const end = new Date(event.end);

    if (!event.recurrence || event.recurrence.type === 'none') {
      // Single event - check if it falls within range
      if (start <= endDate && end >= startDate) {
        occurrences.push({
          event: event,
          start: new Date(start),
          end: new Date(end),
          isException: false
        });
      }
      return occurrences;
    }

    const recurrence = event.recurrence;
    let currentDate = new Date(start);

    // For weekly: iterate through weekdays
    if (recurrence.type === 'weekly') {
      // Find the first occurrence on or after startDate
      while (currentDate < startDate) {
        currentDate = addDays(currentDate, 1);
      }

      // Generate occurrences until past endDate
      const maxIterations = 366 * 2; // Safety limit (2 years of days)
      let iterations = 0;

      while (currentDate <= endDate && iterations < maxIterations) {
        iterations++;
        if (recurrence.days.includes(currentDate.getDay())) {
          // Check for exception
          const dateKey = formatDateKey(currentDate);
          let occurrenceStart, occurrenceEnd;

          if (event.exceptions && event.exceptions[dateKey]) {
            const exception = event.exceptions[dateKey];
            if (exception.type === 'moved') {
              occurrenceStart = new Date(exception.start);
              occurrenceEnd = new Date(exception.end);
            } else if (exception.type === 'deleted') {
              currentDate = addDays(currentDate, 1);
              continue;
            }
          } else {
            // Create occurrence at same time as original (DST-safe using calendar components)
            occurrenceStart = createDateTime(
              currentDate.getFullYear(),
              currentDate.getMonth(),
              currentDate.getDate(),
              start.getHours(),
              start.getMinutes()
            );
            occurrenceEnd = createDateTime(
              currentDate.getFullYear(),
              currentDate.getMonth(),
              currentDate.getDate(),
              end.getHours(),
              end.getMinutes()
            );
          }

          if (occurrenceStart <= endDate && occurrenceEnd >= startDate) {
            occurrences.push({
              event: event,
              start: occurrenceStart,
              end: occurrenceEnd,
              isException: false
            });
          }
        }
        currentDate = addDays(currentDate, 1);
      }
    }

    // For monthly: iterate through months
    else if (recurrence.type === 'monthly') {
      let year = startDate.getFullYear();
      let month = startDate.getMonth();

      // Find first valid occurrence
      while (year < endDate.getFullYear() || (year === endDate.getFullYear() && month <= endDate.getMonth())) {
        let day = getNthWeekdayOfMonth(year, month, recurrence.occurrence, recurrence.dayOfWeek);

        if (day === null) {
          // Skip months without this occurrence (e.g., no 5th weekday)
          month++;
          if (month > 11) {
            month = 0;
            year++;
          }
          continue;
        }

        const occurrenceDate = new Date(year, month, day);

        // Check for exception
        const dateKey = formatDateKey(occurrenceDate);
        let occurrenceStart, occurrenceEnd;

        if (event.exceptions && event.exceptions[dateKey]) {
          const exception = event.exceptions[dateKey];
          if (exception.type === 'moved') {
            occurrenceStart = new Date(exception.start);
            occurrenceEnd = new Date(exception.end);
          } else if (exception.type === 'deleted') {
            month++;
            if (month > 11) {
              month = 0;
              year++;
            }
            continue;
          }
        } else {
          // Create occurrence at same time as original (DST-safe)
          occurrenceStart = createDateTime(year, month, day, start.getHours(), start.getMinutes());
          occurrenceEnd = createDateTime(year, month, day, end.getHours(), end.getMinutes());
        }

        if (occurrenceStart <= endDate && occurrenceEnd >= startDate) {
          occurrences.push({
            event: event,
            start: occurrenceStart,
            end: occurrenceEnd,
            isException: false
          });
        }

        month++;
        if (month > 11) {
          month = 0;
          year++;
        }
      }
    }

    return occurrences;
  }

  function addDays(date, days) {
    const result = new Date(date);
    result.setDate(result.getDate() + days);
    return result;
  }

  function addMinutes(date, minutes) {
    const result = new Date(date);
    result.setTime(result.getTime() + minutes * 60000);
    return result;
  }

  function snapTo15Minutes(date) {
    const minutes = date.getMinutes();
    const remainder = minutes % 15;
    if (remainder !== 0) {
      date.setMinutes(minutes + (15 - remainder));
    }
    return date;
  }

  // Storage
  function loadEvents() {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (stored) {
      try {
        state.events = JSON.parse(stored);
        return true;
      } catch (e) {
        console.error('Failed to load events:', e);
      }
    }
    return false;
  }

  function saveEvents() {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(state.events));
  }

  // Find event by ID (including recurring series)
  function findEvent(eventId) {
    for (const event of state.events) {
      if (event.id === eventId || event.seriesId === eventId) {
        return event;
      }
    }
    return null;
  }

  // Find series by seriesId
  function findSeries(seriesId) {
    return state.events.find(e => e.seriesId === seriesId) || state.events.find(e => e.id === seriesId);
  }

  // Render calendar
  function render() {
    const grid = document.getElementById('calendarGrid');
    const dateDisplay = document.getElementById('currentDateDisplay');
    grid.className = 'calendar-grid';

    if (state.view === 'month') {
      grid.classList.add('month-view');
      renderMonthView(grid, dateDisplay);
    } else {
      grid.classList.add('week-view');
      renderWeekView(grid, dateDisplay);
    }
  }

  function renderMonthView(container, dateDisplay) {
    const year = state.currentDate.getFullYear();
    const month = state.currentDate.getMonth();

    // Update header
    const monthNames = ['January', 'February', 'March', 'April', 'May', 'June',
                        'July', 'August', 'September', 'October', 'November', 'December'];
    dateDisplay.textContent = `${monthNames[month]} ${year}`;

    // Get all occurrences for this month plus surrounding weeks
    const startOfWeek = getStartOfWeek(new Date(year, month, 1));
    const endOfWeek = new Date(startOfWeek);
    endOfWeek.setDate(endOfWeek.getDate() + 41); // 6 weeks * 7 days - 1

    const allOccurrences = getAllOccurrencesInRange(startOfWeek, endOfWeek);

    // Render week headers
    const dayNames = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];
    for (let i = 0; i < 7; i++) {
      const header = document.createElement('div');
      header.className = 'week-header';
      if (i === 0) header.classList.add('sun');
      if (i === 6) header.classList.add('sat');
      header.textContent = dayNames[i];
      container.appendChild(header);
    }

    // Render 6 weeks (42 days)
    const today = new Date();
    today.setHours(0, 0, 0, 0);

    let currentDate = new Date(startOfWeek);
    for (let day = 0; day < 42; day++) {
      const dayCell = document.createElement('div');
      dayCell.className = 'day-cell';

      const isOtherMonth = currentDate.getMonth() !== month;
      if (isOtherMonth) {
        dayCell.classList.add('other-month');
      }

      const isToday = currentDate.getTime() === today.getTime();
      if (isToday) {
        dayCell.classList.add('today');
      }

      // Day number
      const dayNum = document.createElement('span');
      dayNum.className = 'day-number';
      dayNum.textContent = currentDate.getDate();
      dayCell.appendChild(dayNum);

      // Events for this day
      const dayKey = formatDateKey(currentDate);
      const dayEvents = allOccurrences.filter(o => formatDateKey(o.start) === dayKey);
      dayEvents.sort((a, b) => a.start - b.start);

      const eventsContainer = document.createElement('div');
      eventsContainer.className = 'day-events';

      for (const occurrence of dayEvents) {
        const eventDiv = document.createElement('div');
        eventDiv.className = 'day-event';
        eventDiv.style.backgroundColor = occurrence.event.color;
        eventDiv.textContent = formatTimeRange(occurrence.start, occurrence.end);
        eventDiv.title = occurrence.event.title;
        eventDiv.addEventListener('click', (e) => {
          e.stopPropagation();
          openEventDialog(occurrence.event, occurrence);
        });
        eventsContainer.appendChild(eventDiv);
      }

      dayCell.appendChild(eventsContainer);

      // Click to create all-day event
      dayCell.addEventListener('click', () => {
        createAllDayEvent(currentDate);
      });

      container.appendChild(dayCell);
      currentDate.setDate(currentDate.getDate() + 1);
    }
  }

  function renderWeekView(container, dateDisplay) {
    const weekStart = getStartOfWeek(state.currentDate);
    const weekEnd = new Date(weekStart);
    weekEnd.setDate(weekEnd.getDate() + 6);

    // Update header
    const monthNames = ['January', 'February', 'March', 'April', 'May', 'June',
                        'July', 'August', 'September', 'October', 'November', 'December'];
    if (weekStart.getMonth() === weekEnd.getMonth()) {
      dateDisplay.textContent = `${monthNames[weekStart.getMonth()]} ${weekStart.getFullYear()}`;
    } else {
      dateDisplay.textContent = `${monthNames[weekStart.getMonth()]} - ${monthNames[weekEnd.getMonth()]} ${weekEnd.getFullYear()}`;
    }

    // Time header
    const timeHeader = document.createElement('div');
    timeHeader.className = 'time-header';

    const corner = document.createElement('div');
    corner.className = 'corner';
    timeHeader.appendChild(corner);

    const dayNames = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];
    const today = new Date();
    today.setHours(0, 0, 0, 0);

    for (let i = 0; i < 7; i++) {
      const dayHeader = document.createElement('div');
      dayHeader.className = 'day-header';
      if (i === 0) dayHeader.classList.add('sun');
      if (i === 6) dayHeader.classList.add('sat');

      const dayDate = new Date(weekStart);
      dayDate.setDate(dayDate.getDate() + i);

      if (dayDate.getTime() === today.getTime()) {
        dayHeader.classList.add('today');
      }

      dayHeader.textContent = `${dayNames[i]} ${dayDate.getDate()}`;
      timeHeader.appendChild(dayHeader);
    }
    container.appendChild(timeHeader);

    // Get all occurrences for this week
    const todayStart = new Date(weekStart);
    todayStart.setHours(0, 0, 0, 0);
    const todayEnd = new Date(todayStart);
    todayEnd.setDate(todayEnd.getDate() + 7);
    const allOccurrences = getAllOccurrencesInRange(todayStart, todayEnd);

    // Time grid
    const timeGrid = document.createElement('div');
    timeGrid.className = 'time-grid';

    // Time labels
    const timeLabels = document.createElement('div');
    timeLabels.className = 'time-labels';
    for (let h = 0; h < 24; h++) {
      const label = document.createElement('div');
      label.className = 'time-label';
      label.textContent = formatHour(h);
      timeLabels.appendChild(label);
    }
    timeGrid.appendChild(timeLabels);

    // Days container
    const daysContainer = document.createElement('div');
    daysContainer.className = 'days-container';

    for (let d = 0; d < 7; d++) {
      const dayColumn = document.createElement('div');
      dayColumn.className = 'day-column';
      dayColumn.dataset.dayIndex = d;

      const dayDate = new Date(weekStart);
      dayDate.setDate(dayDate.getDate() + d);

      if (dayDate.getTime() === today.getTime()) {
        dayColumn.classList.add('today');
      }

      // Hour rows (drag zones)
      for (let h = 0; h < 24; h++) {
        const hourRow = document.createElement('div');
        hourRow.className = 'hour-row drag-zone';
        hourRow.dataset.hour = h;
        hourRow.dataset.dayDate = formatDateKey(dayDate);

        // Mouse down for drag-to-create
        hourRow.addEventListener('mousedown', (e) => {
          if (e.button === 0) { // Left click only
            startDragCreate(e, dayDate, h);
          }
        });

        dayColumn.appendChild(hourRow);
      }

      // Add events to this day
      const dayKey = formatDateKey(dayDate);
      const dayEvents = allOccurrences.filter(o => formatDateKey(o.start) === dayKey);
      renderDayEvents(dayColumn, dayEvents);

      daysContainer.appendChild(dayColumn);
    }

    timeGrid.appendChild(daysContainer);
    container.appendChild(timeGrid);
  }

  function renderDayEvents(dayColumn, events) {
    // Sort events by start time
    events.sort((a, b) => a.start - b.start);

    // Calculate overlapping groups
    const groups = [];
    let currentGroup = [];
    let currentEnd = null;

    for (const event of events) {
      if (currentEnd && event.start >= currentEnd) {
        groups.push(currentGroup);
        currentGroup = [];
        currentEnd = null;
      }
      currentGroup.push(event);
      if (!currentEnd || event.end > currentEnd) {
        currentEnd = event.end;
      }
    }
    if (currentGroup.length > 0) {
      groups.push(currentGroup);
    }

    // Render each group with appropriate width
    for (const group of groups) {
      const groupWidth = 100 / group.length;
      let groupLeft = 0;

      for (let i = 0; i < group.length; i++) {
        renderEvent(dayColumn, group[i], groupLeft, groupWidth);
        groupLeft += groupWidth;
      }
    }
  }

  function renderEvent(dayColumn, occurrence, leftPercent, widthPercent) {
    const eventDiv = document.createElement('div');
    eventDiv.className = 'event';
    if (occurrence.isException) {
      eventDiv.classList.add('exception');
    }

    eventDiv.style.backgroundColor = occurrence.event.color;
    eventDiv.style.left = leftPercent + '%';
    eventDiv.style.width = (widthPercent - 2) + '%'; // -2 for border

    // Calculate position based on time
    const startMinutes = occurrence.start.getHours() * 60 + occurrence.start.getMinutes();
    const endMinutes = occurrence.end.getHours() * 60 + occurrence.end.getMinutes();
    const duration = endMinutes - startMinutes;

    eventDiv.style.top = (startMinutes) + 'px'; // 40px per hour, but we'll use 1px per minute for precision
    eventDiv.style.height = Math.max(20, duration) + 'px';

    eventDiv.innerHTML = `
      <div class="event-title">${occurrence.event.title}</div>
      <div class="event-time">${formatTimeRange(occurrence.start, occurrence.end)}</div>
      <div class="resize-handle"></div>
    `;

    // Click to edit
    eventDiv.addEventListener('click', (e) => {
      if (!e.target.classList.contains('resize-handle')) {
        openEventDialog(occurrence.event, occurrence);
      }
    });

    // Drag to move
    eventDiv.addEventListener('mousedown', (e) => {
      if (e.target.classList.contains('resize-handle')) {
        startResize(e, occurrence, eventDiv);
      } else if (e.button === 0) {
        startDragEvent(e, occurrence, eventDiv);
      }
    });

    dayColumn.appendChild(eventDiv);
  }

  function getAllOccurrencesInRange(startDate, endDate) {
    const allOccurrences = [];

    for (const event of state.events) {
      const occurrences = expandRecurrence(event, startDate, endDate);
      allOccurrences.push(...occurrences);
    }

    return allOccurrences;
  }

  // Drag to create event
  function startDragCreate(e, dayDate, startHour) {
    e.preventDefault();
    e.stopPropagation();

    const column = e.target.closest('.day-column');
    if (!column) return;

    const startY = e.clientY;
    const rowHeight = 40; // pixels per hour

    const dragZone = e.target.closest('.hour-row');
    const startMinutes = startHour * 60;

    let currentMinutes = startMinutes;

    function onMouseMove(moveEvent) {
      const deltaY = moveEvent.clientY - startY;
      const deltaMinutes = Math.round(deltaY / rowHeight * 60);
      let endMinutes = startMinutes + deltaMinutes;

      // Snap to 15 minutes
      endMinutes = Math.round(endMinutes / 15) * 15;

      // Ensure end is after start
      if (endMinutes <= startMinutes) {
        endMinutes = startMinutes + 15;
      }

      // Clamp to day bounds
      endMinutes = Math.min(endMinutes, 24 * 60);

      const start = new Date(dayDate);
      start.setMinutes(startMinutes);

      const end = new Date(dayDate);
      end.setMinutes(endMinutes);

      // Update drag preview
      updateDragPreview(column, startMinutes, endMinutes);
    }

    function onMouseUp() {
      document.removeEventListener('mousemove', onMouseMove);
      document.removeEventListener('mouseup', onMouseUp);

      // Remove preview
      const preview = document.querySelector('.drag-preview');
      if (preview) preview.remove();

      // Create event
      const start = new Date(dayDate);
      start.setMinutes(startMinutes);

      const previewEl = document.querySelector('.drag-preview');
      if (previewEl) {
        const endMinutes = parseInt(previewEl.dataset.endMinutes);
        const end = new Date(dayDate);
        end.setMinutes(endMinutes);

        createEvent(start, end);
      }

      // Clear drag state
      state.dragState = null;
    }

    document.addEventListener('mousemove', onMouseMove);
    document.addEventListener('mouseup', onMouseUp);

    state.dragState = {
      type: 'create',
      dayDate: dayDate,
      startMinutes: startMinutes
    };
  }

  function updateDragPreview(column, startMinutes, endMinutes) {
    let preview = document.querySelector('.drag-preview');
    if (!preview) {
      preview = document.createElement('div');
      preview.className = 'drag-preview';
      column.appendChild(preview);
    }

    preview.dataset.endMinutes = endMinutes;
    preview.style.top = startMinutes + 'px';
    preview.style.height = (endMinutes - startMinutes) + 'px';
  }

  // Drag event to move
  function startDragEvent(e, occurrence, eventEl) {
    e.preventDefault();
    e.stopPropagation();

    const column = eventEl.closest('.day-column');
    const dayDate = new Date(column.querySelector('.hour-row').dataset.dayDate);

    const startY = e.clientY;
    const originalStart = new Date(occurrence.start);
    const originalDayKey = formatDateKey(originalStart);

    let currentDayDate = new Date(dayDate);
    let currentMinutes = originalStart.getHours() * 60 + originalStart.getMinutes();

    function onMouseMove(moveEvent) {
      const deltaY = moveEvent.clientY - startY;
      const rowHeight = 40;
      const deltaMinutes = Math.round(deltaY / rowHeight * 60);

      let newMinutes = currentMinutes + deltaMinutes;

      // Snap to 15 minutes
      newMinutes = Math.round(newMinutes / 15) * 15;

      // Clamp to day bounds
      if (newMinutes < 0) newMinutes = 0;
      if (newMinutes > 24 * 60 - 15) newMinutes = 24 * 60 - 15;

      eventEl.classList.add('dragging');
      eventEl.style.top = newMinutes + 'px';
    }

    function onMouseUp() {
      document.removeEventListener('mousemove', onMouseMove);
      document.removeEventListener('mouseup', onMouseUp);

      eventEl.classList.remove('dragging');

      // Calculate new time
      const newMinutes = parseInt(eventEl.style.top);
      const duration = (new Date(occurrence.end) - new Date(occurrence.start)) / 60000;

      const newStart = new Date(dayDate);
      newStart.setMinutes(newMinutes);

      const newEnd = new Date(newStart);
      newEnd.setMinutes(newMinutes + duration);

      // Update event
      updateEventTime(occurrence.event, occurrence, newStart, newEnd);

      render();
    }

    document.addEventListener('mousemove', onMouseMove);
    document.addEventListener('mouseup', onMouseUp);
  }

  // Resize event
  function startResize(e, occurrence, eventEl) {
    e.preventDefault();
    e.stopPropagation();

    const startY = e.clientY;
    const originalEnd = new Date(occurrence.end);

    function onMouseMove(moveEvent) {
      const deltaY = moveEvent.clientY - startY;
      const rowHeight = 40;
      const deltaMinutes = Math.round(deltaY / rowHeight * 60);

      let newEndMinutes = (originalEnd.getHours() * 60 + originalEnd.getMinutes()) + deltaMinutes;

      // Snap to 15 minutes
      newEndMinutes = Math.round(newEndMinutes / 15) * 15;

      // Ensure minimum duration
      const startMinutes = occurrence.start.getHours() * 60 + occurrence.start.getMinutes();
      if (newEndMinutes <= startMinutes) {
        newEndMinutes = startMinutes + 15;
      }

      eventEl.style.height = (newEndMinutes - startMinutes) + 'px';
    }

    function onMouseUp() {
      document.removeEventListener('mousemove', onMouseMove);
      document.removeEventListener('mouseup', onMouseUp);

      const newEndMinutes = parseInt(eventEl.style.top) + parseInt(eventEl.style.height);
      const newEnd = new Date(occurrence.start);
      newEnd.setMinutes(newEndMinutes);

      // Update event end time
      updateEventEndTime(occurrence.event, occurrence, newEnd);

      render();
    }

    document.addEventListener('mousemove', onMouseMove);
    document.addEventListener('mouseup', onMouseUp);
  }

  // Event management
  function createAllDayEvent(date) {
    const start = new Date(date);
    start.setHours(9, 0, 0, 0);
    const end = new Date(date);
    end.setHours(10, 0, 0, 0);
    createEvent(start, end);
  }

  function createEvent(start, end) {
    const event = {
      id: generateId(),
      seriesId: null,
      title: 'New Event',
      start: start,
      end: end,
      color: '#3498db',
      recurrence: { type: 'none' },
      exceptions: {}
    };
    event.seriesId = event.id;

    state.events.push(event);
    saveEvents();
    openEventDialog(event, { event: event, start: start, end: end });
  }

  function updateEventTime(event, occurrence, newStart, newEnd) {
    if (event.recurrence && event.recurrence.type !== 'none') {
      // For recurring events, ask what to update
      showDeleteDialog(() => {
        if (state.deleteMode === 'this') {
          // Create exception for this occurrence
          const dateKey = formatDateKey(newStart);
          event.exceptions[dateKey] = {
            type: 'moved',
            start: newStart,
            end: newEnd
          };
        } else {
          // Update the whole series (change original time)
          event.start = newStart;
          event.end = newEnd;
        }
        saveEvents();
        render();
      });
    } else {
      event.start = newStart;
      event.end = newEnd;
      saveEvents();
      render();
    }
  }

  function updateEventEndTime(event, occurrence, newEnd) {
    if (event.recurrence && event.recurrence.type !== 'none') {
      showDeleteDialog(() => {
        if (state.deleteMode === 'this') {
          const dateKey = formatDateKey(occurrence.start);
          event.exceptions[dateKey] = {
            type: 'moved',
            start: new Date(occurrence.start),
            end: newEnd
          };
        } else {
          event.end = newEnd;
        }
        saveEvents();
        render();
      });
    } else {
      event.end = newEnd;
      saveEvents();
      render();
    }
  }

  function deleteEvent(event, occurrence) {
    if (event.recurrence && event.recurrence.type !== 'none') {
      showDeleteDialog(() => {
        if (state.deleteMode === 'this') {
          // Mark this occurrence as deleted
          const dateKey = formatDateKey(occurrence.start);
          event.exceptions[dateKey] = { type: 'deleted' };
        } else {
          // Remove the whole series
          state.events = state.events.filter(e => e.id !== event.id && e.seriesId !== event.id);
        }
        saveEvents();
        render();
      });
    } else {
      state.events = state.events.filter(e => e.id !== event.id);
      saveEvents();
      render();
    }
  }

  function saveEvent(formData) {
    const event = state.editingEventId ? findEvent(state.editingEventId) : null;

    if (event) {
      event.title = formData.title;
      event.color = formData.color;
      event.recurrence = formData.recurrence;

      if (!event.recurrence || event.recurrence.type === 'none') {
        event.start = formData.start;
        event.end = formData.end;
      }

      saveEvents();
    }
    render();
  }

  // Dialog handling
  function openEventDialog(event, occurrence) {
    const dialog = document.getElementById('eventDialog');
    const form = document.getElementById('eventForm');
    const dialogTitle = document.getElementById('dialogTitle');
    const deleteBtn = document.getElementById('deleteEventBtn');

    state.editingEventId = event.id;
    state.editingEventSeriesId = event.seriesId;

    dialogTitle.textContent = event.recurrence && event.recurrence.type !== 'none' ? 'Edit Recurring Event' : 'Edit Event';
    deleteBtn.classList.remove('hidden');

    document.getElementById('eventTitle').value = event.title;
    document.getElementById('eventStart').value = formatDateTime(event.start);
    document.getElementById('eventEnd').value = formatDateTime(event.end);
    document.getElementById('eventColor').value = event.color;

    const recurrenceType = (event.recurrence && event.recurrence.type) || 'none';
    document.getElementById('recurrenceType').value = recurrenceType;

    toggleRecurrenceOptions();

    // Set weekday checkboxes for weekly
    if (recurrenceType === 'weekly' && event.recurrence.days) {
      document.querySelectorAll('#weeklyOptions input[type="checkbox"]').forEach(cb => {
        cb.checked = event.recurrence.days.includes(parseInt(cb.value));
      });
    }

    // Set monthly options
    if (recurrenceType === 'monthly') {
      document.getElementById('monthlyOccurrence').value = event.recurrence.occurrence || 1;
      document.getElementById('monthlyDay').value = event.recurrence.dayOfWeek || 0;
    }

    dialog.classList.remove('hidden');

    // Validation: end after start
    const startInput = document.getElementById('eventStart');
    const endInput = document.getElementById('eventEnd');

    function validateTime() {
      const start = new Date(startInput.value);
      const end = new Date(endInput.value);
      if (end <= start) {
        endInput.setCustomValidity('End time must be after start time');
      } else {
        endInput.setCustomValidity('');
      }
    }

    startInput.addEventListener('change', validateTime);
    endInput.addEventListener('change', validateTime);
  }

  function closeEventDialog() {
    document.getElementById('eventDialog').classList.add('hidden');
    document.getElementById('eventForm').reset();
    state.editingEventId = null;
    state.editingEventSeriesId = null;
  }

  function showDeleteDialog(onConfirm) {
    state.deleteMode = null;
    const dialog = document.getElementById('deleteDialog');
    dialog.classList.remove('hidden');

    const deleteThisBtn = document.getElementById('deleteThisBtn');
    const deleteAllBtn = document.getElementById('deleteAllBtn');

    deleteThisBtn.onclick = () => {
      state.deleteMode = 'this';
      dialog.classList.add('hidden');
      onConfirm();
    };

    deleteAllBtn.onclick = () => {
      state.deleteMode = 'all';
      dialog.classList.add('hidden');
      onConfirm();
    };

    document.getElementById('cancelDeleteBtn').onclick = () => {
      dialog.classList.add('hidden');
    };
  }

  function toggleRecurrenceOptions() {
    const type = document.getElementById('recurrenceType').value;
    const weeklyOpts = document.getElementById('weeklyOptions');
    const monthlyOpts = document.getElementById('monthlyOptions');

    weeklyOpts.classList.add('hidden');
    monthlyOpts.classList.add('hidden');

    if (type === 'weekly') {
      weeklyOpts.classList.remove('hidden');
    } else if (type === 'monthly') {
      monthlyOpts.classList.remove('hidden');
    }
  }

  // Navigation
  function goToPrev() {
    if (state.view === 'month') {
      state.currentDate.setMonth(state.currentDate.getMonth() - 1);
    } else {
      state.currentDate.setDate(state.currentDate.getDate() - 7);
    }
    render();
  }

  function goToNext() {
    if (state.view === 'month') {
      state.currentDate.setMonth(state.currentDate.getMonth() + 1);
    } else {
      state.currentDate.setDate(state.currentDate.getDate() + 7);
    }
    render();
  }

  function goToToday() {
    state.currentDate = new Date();
    render();
  }

  function setView(view) {
    state.view = view;
    document.getElementById('monthViewBtn').classList.toggle('active', view === 'month');
    document.getElementById('weekViewBtn').classList.toggle('active', view === 'week');
    render();
  }

  // Helper functions
  function getStartOfWeek(date) {
    const result = new Date(date);
    const day = result.getDay();
    result.setDate(result.getDate() - day);
    result.setHours(0, 0, 0, 0);
    return result;
  }

  function formatHour(hour) {
    if (hour === 0) return '12 AM';
    if (hour === 12) return '12 PM';
    if (hour < 12) return hour + ' AM';
    return (hour - 12) + ' PM';
  }

  function formatTimeRange(start, end) {
    const startHour = start.getHours();
    const startMin = start.getMinutes();
    const endHour = end.getHours();
    const endMin = end.getMinutes();

    const startStr = formatTime(startHour, startMin);
    const endStr = formatTime(endHour, endMin);

    return `${startStr} - ${endStr}`;
  }

  function formatTime(hour, min) {
    const ampm = hour >= 12 ? 'PM' : 'AM';
    const displayHour = hour % 12 || 12;
    const displayMin = String(min).padStart(2, '0');
    return `${displayHour}:${displayMin} ${ampm}`;
  }

  // Event listeners
  function initEventListeners() {
    document.getElementById('prevBtn').addEventListener('click', goToPrev);
    document.getElementById('nextBtn').addEventListener('click', goToNext);
    document.getElementById('todayBtn').addEventListener('click', goToToday);
    document.getElementById('monthViewBtn').addEventListener('click', () => setView('month'));
    document.getElementById('weekViewBtn').addEventListener('click', () => setView('week'));

    document.getElementById('recurrenceType').addEventListener('change', toggleRecurrenceOptions);

    document.getElementById('saveEventBtn').addEventListener('click', () => {
      const form = document.getElementById('eventForm');
      if (!form.checkValidity()) {
        form.reportValidity();
        return;
      }

      const recurrenceType = document.getElementById('recurrenceType').value;
      const recurrence = { type: recurrenceType };

      if (recurrenceType === 'weekly') {
        const days = Array.from(document.querySelectorAll('#weeklyOptions input[type="checkbox"]:checked'))
          .map(cb => parseInt(cb.value));
        if (days.length === 0) {
          alert('Please select at least one day of the week');
          return;
        }
        recurrence.days = days;
      } else if (recurrenceType === 'monthly') {
        recurrence.occurrence = parseInt(document.getElementById('monthlyOccurrence').value);
        recurrence.dayOfWeek = parseInt(document.getElementById('monthlyDay').value);
      }

      const formData = {
        title: document.getElementById('eventTitle').value,
        start: new Date(document.getElementById('eventStart').value),
        end: new Date(document.getElementById('eventEnd').value),
        color: document.getElementById('eventColor').value,
        recurrence: recurrence
      };

      saveEvent(formData);
      closeEventDialog();
    });

    document.getElementById('cancelEventBtn').addEventListener('click', closeEventDialog);

    document.getElementById('deleteEventBtn').addEventListener('click', () => {
      const event = state.editingEventId ? findEvent(state.editingEventId) : null;
      if (event) {
        closeEventDialog();
        deleteEvent(event, null);
      }
    });

    document.getElementById('eventDialog').addEventListener('click', (e) => {
      if (e.target.id === 'eventDialog') {
        closeEventDialog();
      }
    });
  }

  // Initialize
  function init() {
    initEventListeners();

    // Load events or demo data
    if (!loadEvents() || state.events.length === 0) {
      loadDemoData();
    }

    render();
  }

  // Start the app
  init();
})();
