(function () {
  const LS_TOKEN = 'mw_token';
  const LS_CODE = 'mw_code';

  const state = {
    token: null,
    code: null,
    displayName: '',
    memberID: null,
    joinCode: null,
    room: null,
    buffer: [],
    history: [],
    activeTab: 'swipe',
    genres: [],
    selectedGenreIds: new Set(),
  };

  function $(id) {
    return document.getElementById(id);
  }

  function show(id) {
    $(id).classList.remove('hidden');
  }

  function hide(id) {
    $(id).classList.add('hidden');
  }

  async function api(path, opts) {
    opts = opts || {};
    const headers = Object.assign({ 'Content-Type': 'application/json' }, opts.headers || {});
    if (state.token) headers['X-Member-Token'] = state.token;
    const res = await fetch('/api' + path, Object.assign({}, opts, { headers }));
    if (!res.ok) {
      let msg = res.statusText;
      try {
        const body = await res.json();
        if (body.error) msg = body.error;
      } catch (e) {}
      const err = new Error(msg);
      err.status = res.status;
      throw err;
    }
    if (res.status === 204) return null;
    return res.json();
  }

  function posterURL(path, size) {
    return `https://image.tmdb.org/t/p/${size || 'w500'}${path}`;
  }

  function setOnboardingError(msg) {
    if (!msg) {
      hide('onboarding-error');
      return;
    }
    $('onboarding-error').textContent = msg;
    show('onboarding-error');
  }

  function initOnboarding() {
    const params = new URLSearchParams(location.search);
    state.joinCode = params.get('join');

    $('input-name').addEventListener('input', () => {
      $('btn-continue-name').disabled = $('input-name').value.trim().length === 0;
    });

    $('btn-continue-name').addEventListener('click', () => {
      state.displayName = $('input-name').value.trim();
      if (!state.displayName) return;
      hide('step-name');
      setOnboardingError(null);
      if (state.joinCode) {
        $('input-code').value = state.joinCode.toUpperCase();
        show('step-join');
      } else {
        show('step-choice');
      }
    });

    $('btn-show-create').addEventListener('click', () => {
      hide('step-choice');
      show('step-create');
    });

    $('btn-show-join').addEventListener('click', () => {
      hide('step-choice');
      show('step-join');
    });

    document.querySelectorAll('[data-back]').forEach((btn) => {
      btn.addEventListener('click', () => {
        hide('step-create');
        hide('step-join');
        setOnboardingError(null);
        show(btn.dataset.back);
      });
    });

    $('btn-create').addEventListener('click', async () => {
      setOnboardingError(null);
      try {
        const resp = await api('/rooms', {
          method: 'POST',
          body: JSON.stringify({
            name: $('input-room-name').value.trim(),
            display_name: state.displayName,
            filters: {},
          }),
        });
        persistSession(resp.token, resp.code);
        enterApp(resp.room);
      } catch (e) {
        setOnboardingError(e.message || 'Could not create room');
      }
    });

    $('btn-join').addEventListener('click', async () => {
      setOnboardingError(null);
      const code = $('input-code').value.trim().toUpperCase();
      if (code.length !== 6) {
        setOnboardingError('Enter the 6-character room code');
        return;
      }
      try {
        const resp = await api(`/rooms/${code}/join`, {
          method: 'POST',
          body: JSON.stringify({ display_name: state.displayName }),
        });
        persistSession(resp.token, code);
        enterApp(resp.room);
      } catch (e) {
        setOnboardingError(e.message || 'Could not join room');
      }
    });
  }

  function persistSession(token, code) {
    state.token = token;
    state.code = code;
    localStorage.setItem(LS_TOKEN, token);
    localStorage.setItem(LS_CODE, code);
  }

  function clearSession() {
    state.token = null;
    state.code = null;
    localStorage.removeItem(LS_TOKEN);
    localStorage.removeItem(LS_CODE);
  }

  function enterApp(room) {
    state.room = room;
    state.memberID = room.you_id;
    hide('view-onboarding');
    show('view-app');
    renderRoom(room);
    updateUnseenBadge(room.unseen_matches || 0);
    switchTab('swipe');
    refillBuffer();
  }

  function switchTab(tab) {
    state.activeTab = tab;
    document.querySelectorAll('.tab-panel').forEach((p) => p.classList.add('hidden'));
    show('view-' + tab);
    document.querySelectorAll('.tab-btn').forEach((b) => b.classList.toggle('active', b.dataset.tab === tab));
    if (tab === 'lists') loadLists();
    if (tab === 'room') renderRoom(state.room);
  }

  document.querySelectorAll('.tab-btn').forEach((b) => b.addEventListener('click', () => switchTab(b.dataset.tab)));

  function updateUnseenBadge(count) {
    const badge = $('lists-badge');
    const banner = $('swipe-banner');
    if (count > 0) {
      badge.textContent = String(count);
      show('lists-badge');
      banner.textContent = `${count} new match${count === 1 ? '' : 'es'} — check Lists`;
      show('swipe-banner');
    } else {
      hide('lists-badge');
      hide('swipe-banner');
    }
  }

  const stack = $('card-stack');

  function renderStack() {
    stack.querySelectorAll('.card').forEach((el) => el.remove());
    if (state.buffer.length === 0) {
      show('empty-state');
      return;
    }
    hide('empty-state');
    const visible = state.buffer.slice(0, 2);
    for (let i = visible.length - 1; i >= 0; i--) {
      const el = buildCardElement(visible[i], i === 0);
      if (i === 1) el.classList.add('card-peek');
      stack.appendChild(el);
    }
  }

  function buildCardElement(movie, isTop) {
    const card = document.createElement('div');
    card.className = 'card';

    if (movie.poster_path) {
      const img = document.createElement('img');
      img.className = 'card-poster';
      img.src = posterURL(movie.poster_path);
      img.alt = movie.title;
      img.draggable = false;
      card.appendChild(img);
    } else {
      const placeholder = document.createElement('div');
      placeholder.className = 'card-poster card-poster-placeholder';
      placeholder.textContent = movie.title;
      card.appendChild(placeholder);
    }

    const info = document.createElement('div');
    info.className = 'card-info';

    const title = document.createElement('div');
    title.className = 'card-title';
    title.textContent = movie.title + (movie.year ? ` (${movie.year})` : '');
    info.appendChild(title);

    if (movie.genres && movie.genres.length) {
      const chips = document.createElement('div');
      chips.className = 'genre-chips';
      movie.genres.slice(0, 3).forEach((g) => {
        const chip = document.createElement('span');
        chip.className = 'chip';
        chip.textContent = g;
        chips.appendChild(chip);
      });
      info.appendChild(chips);
    }

    if (movie.runtime) {
      const meta = document.createElement('div');
      meta.className = 'card-meta';
      meta.textContent = `${movie.runtime} min`;
      info.appendChild(meta);
    }

    const ratings = document.createElement('div');
    ratings.className = 'ratings-row';
    if (movie.tmdb_rating) ratings.appendChild(ratingPill('TMDB', movie.tmdb_rating.toFixed(1)));
    if (movie.imdb_rating) ratings.appendChild(ratingPill('IMDb', movie.imdb_rating));
    if (movie.rt_rating) ratings.appendChild(ratingPill('RT', movie.rt_rating));
    if (ratings.children.length) info.appendChild(ratings);

    if (movie.overview) {
      const overview = document.createElement('p');
      const isLong = movie.overview.length > 320;
      overview.className = isLong ? 'card-overview clamped' : 'card-overview';
      overview.textContent = movie.overview;
      overview.addEventListener('pointerdown', (e) => e.stopPropagation());
      overview.addEventListener('click', (e) => {
        e.stopPropagation();
        overview.classList.toggle('expanded');
      });
      info.appendChild(overview);
    }

    if (movie.trailer_key) {
      const trailerBtn = document.createElement('button');
      trailerBtn.className = 'btn-trailer';
      trailerBtn.textContent = '▶ Trailer';
      trailerBtn.addEventListener('pointerdown', (e) => e.stopPropagation());
      trailerBtn.addEventListener('click', (e) => {
        e.stopPropagation();
        openTrailer(movie.trailer_key);
      });
      info.appendChild(trailerBtn);
    }

    card.appendChild(info);

    if (isTop) {
      const likeBadge = document.createElement('div');
      likeBadge.className = 'swipe-badge badge-like';
      likeBadge.textContent = 'LIKE';
      const nopeBadge = document.createElement('div');
      nopeBadge.className = 'swipe-badge badge-nope';
      nopeBadge.textContent = 'NOPE';
      card.appendChild(likeBadge);
      card.appendChild(nopeBadge);
      attachDrag(card, movie);
    }

    return card;
  }

  function ratingPill(label, value) {
    const pill = document.createElement('span');
    pill.className = 'rating-pill';
    pill.textContent = `${label} ${value}`;
    return pill;
  }

  function attachDrag(cardEl, movie) {
    let dragging = false;
    let startX = 0;
    let startY = 0;
    let dx = 0;
    let dy = 0;
    let lastMoveTime = 0;
    let lastDx = 0;
    let velocity = 0;

    const likeBadge = cardEl.querySelector('.badge-like');
    const nopeBadge = cardEl.querySelector('.badge-nope');
    cardEl.style.touchAction = 'none';

    cardEl.addEventListener('pointerdown', (e) => {
      dragging = true;
      startX = e.clientX;
      startY = e.clientY;
      lastMoveTime = performance.now();
      lastDx = 0;
      cardEl.setPointerCapture(e.pointerId);
      cardEl.classList.add('dragging');
    });

    cardEl.addEventListener('pointermove', (e) => {
      if (!dragging) return;
      dx = e.clientX - startX;
      dy = e.clientY - startY;
      const now = performance.now();
      const dt = Math.max(now - lastMoveTime, 1);
      velocity = (dx - lastDx) / dt;
      lastDx = dx;
      lastMoveTime = now;

      const rotate = dx / 20;
      cardEl.style.transform = `translate(${dx}px, ${dy}px) rotate(${rotate}deg)`;
      const progress = Math.min(Math.abs(dx) / (window.innerWidth * 0.35), 1);
      if (dx > 0) {
        likeBadge.style.opacity = String(progress);
        nopeBadge.style.opacity = '0';
      } else {
        nopeBadge.style.opacity = String(progress);
        likeBadge.style.opacity = '0';
      }
    });

    function endDrag() {
      if (!dragging) return;
      dragging = false;
      cardEl.classList.remove('dragging');
      const threshold = window.innerWidth * 0.35;
      const fast = Math.abs(velocity) > 0.8 && Math.abs(dx) > 80;
      if (Math.abs(dx) > threshold || fast) {
        commitSwipe(cardEl, movie, dx > 0, dy);
      } else {
        cardEl.style.transition = 'transform 0.2s ease-out';
        cardEl.style.transform = '';
        likeBadge.style.opacity = '0';
        nopeBadge.style.opacity = '0';
        setTimeout(() => {
          cardEl.style.transition = '';
        }, 200);
      }
      dx = 0;
      dy = 0;
      velocity = 0;
    }

    cardEl.addEventListener('pointerup', endDrag);
    cardEl.addEventListener('pointercancel', endDrag);
  }

  function submitSwipe(tmdbId, liked, attempt) {
    attempt = attempt || 0;
    api(`/rooms/${state.code}/swipes`, {
      method: 'POST',
      keepalive: true,
      body: JSON.stringify({ tmdb_id: tmdbId, liked: liked }),
    }).catch(() => {
      if (attempt < 2) {
        setTimeout(() => submitSwipe(tmdbId, liked, attempt + 1), 300 * Math.pow(2, attempt));
      }
    });
  }

  let swipeBusy = false;

  function commitSwipe(cardEl, movie, liked, dy) {
    if (swipeBusy) return;
    swipeBusy = true;
    const flyX = (liked ? 1 : -1) * window.innerWidth * 1.2;
    cardEl.style.transition = 'transform 0.3s ease-out';
    cardEl.style.transform = `translate(${flyX}px, ${dy || 0}px) rotate(${liked ? 20 : -20}deg)`;
    setTimeout(() => {
      cardEl.remove();
      swipeBusy = false;
    }, 300);

    state.history.push(movie);
    if (state.history.length > 20) state.history.shift();
    state.buffer.shift();
    $('btn-undo').disabled = false;

    submitSwipe(movie.tmdb_id, liked);

    renderStack();
    maybeRefill();
  }

  function swipeTop(liked) {
    const top = stack.querySelector('.card:last-child');
    if (!top || state.buffer.length === 0) return;
    commitSwipe(top, state.buffer[0], liked, 0);
  }

  $('btn-like').addEventListener('click', () => swipeTop(true));
  $('btn-pass').addEventListener('click', () => swipeTop(false));

  $('btn-undo').addEventListener('click', async () => {
    const movie = state.history.pop();
    if (!movie) return;
    $('btn-undo').disabled = state.history.length === 0;
    try {
      await api(`/rooms/${state.code}/swipes/${movie.tmdb_id}`, { method: 'DELETE' });
    } catch (e) {}
    state.buffer.unshift(movie);
    renderStack();
  });

  async function refillBuffer() {
    try {
      const cards = await api(`/rooms/${state.code}/cards?limit=10`);
      const existingIds = new Set([
        ...state.buffer.map((m) => m.tmdb_id),
        ...state.history.map((m) => m.tmdb_id),
      ]);
      cards.forEach((c) => {
        if (!existingIds.has(c.tmdb_id)) state.buffer.push(c);
      });
    } catch (e) {}
    renderStack();
    preloadPosters();
  }

  function maybeRefill() {
    if (state.buffer.length < 4) refillBuffer();
  }

  function preloadPosters() {
    state.buffer.slice(0, 3).forEach((m) => {
      if (m.poster_path) {
        const img = new Image();
        img.src = posterURL(m.poster_path);
      }
    });
  }

  function openTrailer(key) {
    $('trailer-frame').src = `https://www.youtube-nocookie.com/embed/${encodeURIComponent(key)}?autoplay=1`;
    show('trailer-modal');
  }

  function closeTrailer() {
    hide('trailer-modal');
    $('trailer-frame').src = '';
  }

  $('btn-close-trailer').addEventListener('click', closeTrailer);
  document.querySelector('#trailer-modal .modal-backdrop').addEventListener('click', closeTrailer);

  async function loadLists() {
    try {
      const data = await api(`/rooms/${state.code}/lists`);
      renderLists(data);
      await api(`/rooms/${state.code}/lists/seen`, { method: 'POST' });
      updateUnseenBadge(0);
    } catch (e) {}
  }

  function renderLists(data) {
    const matchesEl = $('matches-list');
    matchesEl.innerHTML = '';
    (data.matches || []).forEach((m) => matchesEl.appendChild(buildListItem(m)));

    const membersEl = $('members-lists');
    membersEl.innerHTML = '';
    (data.members || []).forEach((member) => {
      const section = document.createElement('div');
      section.className = 'member-list-section';
      const h = document.createElement('h3');
      h.textContent = member.name;
      section.appendChild(h);
      const list = document.createElement('div');
      list.className = 'movie-list';
      const isMine = member.id === state.memberID;
      (member.likes || []).forEach((m) => list.appendChild(buildListItem(m, isMine)));
      section.appendChild(list);
      membersEl.appendChild(section);
    });
  }

  function buildListItem(movie, removable) {
    const item = document.createElement('div');
    item.className = 'list-item';
    if (movie.poster_path) {
      const img = document.createElement('img');
      img.src = posterURL(movie.poster_path, 'w92');
      img.alt = movie.title;
      item.appendChild(img);
    }
    const title = document.createElement('span');
    title.className = 'list-item-title';
    title.textContent = movie.title + (movie.year ? ` (${movie.year})` : '');
    item.appendChild(title);

    if (removable) {
      const removeBtn = document.createElement('button');
      removeBtn.className = 'btn-remove-like';
      removeBtn.type = 'button';
      removeBtn.setAttribute('aria-label', `Remove ${movie.title} from your likes`);
      removeBtn.textContent = '×';
      removeBtn.addEventListener('click', async () => {
        removeBtn.disabled = true;
        try {
          await api(`/rooms/${state.code}/swipes/${movie.tmdb_id}`, { method: 'DELETE' });
          item.remove();
        } catch (e) {
          removeBtn.disabled = false;
        }
      });
      item.appendChild(removeBtn);
    }

    return item;
  }

  function renderRoom(room) {
    if (!room) return;
    $('room-name-display').textContent = room.name || 'Room';
    $('room-code-display').textContent = room.code;
    const list = $('room-members-list');
    list.innerHTML = '';
    (room.members || []).forEach((m) => {
      const li = document.createElement('li');
      const label = document.createElement('span');
      label.textContent = `${m.name} · ${m.swipe_count || 0} swiped`;
      li.appendChild(label);
      if (m.id !== state.memberID) {
        const removeBtn = document.createElement('button');
        removeBtn.className = 'btn-remove-like';
        removeBtn.type = 'button';
        removeBtn.setAttribute('aria-label', `Remove ${m.name} from this room`);
        removeBtn.textContent = '×';
        removeBtn.addEventListener('click', async () => {
          if (!confirm(`Remove ${m.name} from this room? Use this for stray or duplicate members blocking matches.`)) return;
          removeBtn.disabled = true;
          try {
            await api(`/rooms/${state.code}/members/${m.id}`, { method: 'DELETE' });
            const room = await api(`/rooms/${state.code}`);
            state.room = room;
            renderRoom(room);
            loadLists();
          } catch (e) {
            removeBtn.disabled = false;
          }
        });
        li.appendChild(removeBtn);
      }
      list.appendChild(li);
    });
    populateFiltersFromRoom(room);
  }

  async function loadGenres() {
    try {
      state.genres = await api('/genres');
    } catch (e) {
      state.genres = [];
    }
    renderFilterGenreChips();
  }

  function renderFilterGenreChips() {
    const container = $('filter-genre-chips');
    container.innerHTML = '';
    state.genres.forEach((g) => {
      const chip = document.createElement('span');
      chip.className = 'chip';
      chip.textContent = g.name;
      chip.classList.toggle('selected', state.selectedGenreIds.has(g.id));
      chip.addEventListener('click', () => {
        if (state.selectedGenreIds.has(g.id)) {
          state.selectedGenreIds.delete(g.id);
        } else {
          state.selectedGenreIds.add(g.id);
        }
        chip.classList.toggle('selected');
      });
      container.appendChild(chip);
    });
  }

  function populateFiltersFromRoom(room) {
    let filters = {};
    try {
      filters = JSON.parse(room.filters || '{}');
    } catch (e) {}

    $('filter-year-from').value = filters.year_from || '';
    $('filter-year-to').value = filters.year_to || '';
    $('filter-min-rating').value = filters.min_rating || '';
    $('filter-min-votes').value = filters.min_votes || '';
    state.selectedGenreIds = new Set(filters.genre_ids || []);
    renderFilterGenreChips();
  }

  $('btn-save-filters').addEventListener('click', async () => {
    const filters = {};
    const yearFrom = parseInt($('filter-year-from').value, 10);
    const yearTo = parseInt($('filter-year-to').value, 10);
    const minRating = parseFloat($('filter-min-rating').value);
    const minVotes = parseInt($('filter-min-votes').value, 10);
    if (!isNaN(yearFrom)) filters.year_from = yearFrom;
    if (!isNaN(yearTo)) filters.year_to = yearTo;
    if (!isNaN(minRating)) filters.min_rating = minRating;
    if (!isNaN(minVotes)) filters.min_votes = minVotes;
    if (state.selectedGenreIds.size) filters.genre_ids = Array.from(state.selectedGenreIds);

    try {
      await api(`/rooms/${state.code}/filters`, { method: 'PATCH', body: JSON.stringify(filters) });
      state.room.filters = JSON.stringify(filters);
      state.buffer = [];
      state.history = [];
      $('btn-undo').disabled = true;
      renderStack();
      refillBuffer();
      show('filters-saved');
      setTimeout(() => hide('filters-saved'), 2000);
    } catch (e) {}
  });

  function resetOnboarding() {
    state.joinCode = null;
    state.displayName = '';
    setOnboardingError(null);
    $('input-name').value = '';
    $('input-room-name').value = '';
    $('input-code').value = '';
    $('btn-continue-name').disabled = true;
    hide('step-choice');
    hide('step-create');
    hide('step-join');
    show('step-name');
  }

  function leaveRoom() {
    clearSession();
    state.room = null;
    state.memberID = null;
    state.buffer = [];
    state.history = [];
    state.activeTab = 'swipe';
    $('btn-undo').disabled = true;
    hide('lists-badge');
    hide('swipe-banner');
    hide('view-app');
    resetOnboarding();
    show('view-onboarding');
  }

  $('btn-leave-room').addEventListener('click', async () => {
    if (!confirm('Leave this room? You can rejoin later with the room code.')) return;
    try {
      await api(`/rooms/${state.code}/members/${state.memberID}`, { method: 'DELETE' });
    } catch (e) {}
    leaveRoom();
  });

  $('btn-share').addEventListener('click', async () => {
    const url = `${location.origin}/?join=${state.code}`;
    if (navigator.share) {
      try {
        await navigator.share({ title: 'movieswipe', text: `Join my room: ${state.code}`, url });
      } catch (e) {}
    } else {
      try {
        await navigator.clipboard.writeText(url);
        const btn = $('btn-share');
        const original = btn.textContent;
        btn.textContent = 'Copied!';
        setTimeout(() => {
          btn.textContent = original;
        }, 1500);
      } catch (e) {}
    }
  });

  async function init() {
    initOnboarding();
    loadGenres();

    const token = localStorage.getItem(LS_TOKEN);
    const code = localStorage.getItem(LS_CODE);
    if (token && code) {
      state.token = token;
      state.code = code;
      try {
        const room = await api(`/rooms/${code}`);
        enterApp(room);
        return;
      } catch (e) {
        clearSession();
      }
    }
    show('view-onboarding');
  }

  init();
})();
