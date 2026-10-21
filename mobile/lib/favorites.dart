part of 'main.dart';

String favoriteIdentity(String kind, int id) => '$kind:$id';
String favoriteOperationID() {
  final random = Random.secure();
  return base64UrlEncode(
    List.generate(18, (_) => random.nextInt(256)),
  ).replaceAll('=', '');
}

/// Every persisted queue belongs to one endpoint and one account. Request
/// credentials are captured before awaiting, never borrowed from a new session.
class FavoritesStore {
  final AppModel model;
  FavoritesStore(this.model);
  bool active = false;
  final Map<String, Json> _states = {};
  final Map<String, Future<void>> _running = {};
  final Map<String, String> _runningTokens = {};
  final Map<String, String> _errors = {};
  Future<void> _writes = Future.value();
  String get owner => model.token.isEmpty ? 'guest' : '${model.account['id']}';
  String keyFor(String base, String owner) =>
      'favorites.v1.${Uri.encodeComponent(base)}.$owner';
  String get storageKey => keyFor(model.baseUrl, owner);
  String get error => _errors[storageKey] ?? '';
  bool get busy => _running.containsKey(storageKey);
  int get pendingCount => rows(_state(storageKey)['operations']).length;
  Json _state(String key) => _states.putIfAbsent(key, () {
    try {
      final state = asMap(jsonDecode(model.prefs.getString(key) ?? '{}'));
      return {
        'entries': asMap(state['entries'] ?? {}),
        'operations': rows(state['operations']),
      };
    } catch (_) {
      return {'entries': <String, dynamic>{}, 'operations': <Json>[]};
    }
  });
  Json _entries(String key) => asMap(_state(key)['entries']);
  void _setEntries(String key, Json value) => _state(key)['entries'] = value;
  List<Json> get entries => _entries(storageKey).values.map(asMap).toList()
    ..sort(
      (a, b) =>
          (b['collected_at_ms'] as num).compareTo(a['collected_at_ms'] as num),
    );
  bool contains(String kind, int id) =>
      _entries(storageKey).containsKey(favoriteIdentity(kind, id));
  Future<void> _persist(String key) {
    final encoded = jsonEncode(_state(key));
    _writes = _writes.catchError((_) {}).then((_) async {
      if (!await model.prefs.setString(key, encoded)) {
        throw ApiFailure('favoriteSaveFailed');
      }
    });
    return _writes;
  }

  void _notify() => model.favoritesChanged();
  void activate() {
    if (active) return;
    active = true;
    // Run outside build: listeners may rebuild the active navigation tree.
    scheduleMicrotask(() => unawaited(refresh()));
  }

  Future<void> toggle(Json item) async {
    final kind = item['kind'] as String;
    final id = (item['id'] as num).toInt();
    final key = storageKey, target = favoriteIdentity(kind, id);
    final state = _state(key), saved = _entries(key);
    final selected = !saved.containsKey(target);
    final previous = jsonEncode(state);
    if (selected) {
      saved[target] = {
        'kind': kind,
        'content_id': id,
        'collected_at_ms': DateTime.now().millisecondsSinceEpoch,
        'local_id': favoriteOperationID(),
        'available': true,
        'item': item,
      };
    } else {
      saved.remove(target);
    }
    _setEntries(key, saved);
    if (owner != 'guest') {
      state['operations'] = [
        ...rows(state['operations']),
        {
          'operation_id': favoriteOperationID(),
          'kind': kind,
          'content_id': id,
          'favorited': selected,
        },
      ];
    }
    _errors.remove(key);
    _notify();
    try {
      await _persist(key);
    } catch (_) {
      _states[key] = asMap(jsonDecode(previous));
      _errors[key] = 'favoriteSaveFailed';
      _notify();
      rethrow;
    }
    if (active && owner != 'guest') unawaited(refresh());
  }

  Future<void> remove(Json entry) async {
    final target = favoriteIdentity(entry['kind'], entry['content_id']);
    if (!_entries(storageKey).containsKey(target)) return;
    await toggle({'kind': entry['kind'], 'id': entry['content_id']});
  }

  Future<void> mergeGuest() async {
    if (owner == 'guest') return;
    final key = storageKey, guestKey = keyFor(model.baseUrl, 'guest');
    final guest = _entries(guestKey),
        saved = _entries(key),
        state = _state(key);
    final operations = rows(state['operations']);
    var changed = false;
    for (final entry in guest.entries) {
      final row = asMap(entry.value);
      if (row['merge_owner'] != null && row['merge_owner'] != owner) continue;
      if (operations.any((o) => o['guest_local_id'] == row['local_id'])) {
        continue;
      }
      saved.putIfAbsent(entry.key, () => {...row});
      operations.add({
        'operation_id': favoriteOperationID(),
        'kind': row['kind'],
        'content_id': row['content_id'],
        'favorited': true,
        'guest_local_id': row['local_id'],
      });
      guest[entry.key] = {...row, 'merge_owner': owner};
      changed = true;
    }
    if (!changed) return;
    state['operations'] = operations;
    _setEntries(key, saved);
    _setEntries(guestKey, guest);
    // Claim the guest row first to prevent a crash followed by another account
    // login from importing it twice. The same owner can reconstruct its queue.
    await _persist(guestKey);
    await _persist(key);
  }

  Future<Json> _request(
    String base,
    String token,
    String path, {
    Json? body,
  }) async {
    try {
      final request = http.Request(
        body == null ? 'GET' : 'POST',
        Uri.parse('$base/api/v1$path'),
      );
      request.headers['Content-Type'] = 'application/json';
      if (token.isNotEmpty) request.headers['Authorization'] = 'Bearer $token';
      if (body != null) request.body = jsonEncode(body);
      final response = await http.Response.fromStream(
        await model.client.send(request).timeout(const Duration(seconds: 30)),
      ).timeout(const Duration(seconds: 30));
      if (response.statusCode == 404 || response.statusCode == 405) {
        throw ApiFailure('favoritesUpgradeRequired');
      }
      Json parsed;
      try {
        parsed = asMap(jsonDecode(response.body));
      } catch (_) {
        throw ApiFailure('network_error');
      }
      if (response.statusCode >= 400) {
        if (response.statusCode == 401 &&
            model.token == token &&
            model.baseUrl == base) {
          await model.expireSession();
        }
        throw ApiFailure(parsed['error']?['code'] ?? 'internal_error');
      }
      return parsed;
    } on ApiFailure {
      rethrow;
    } catch (_) {
      throw ApiFailure('network_error');
    }
  }

  Future<void> refresh() {
    final key = storageKey;
    final existing = _running[key];
    if (existing != null) {
      final requestedToken = model.token;
      if (_runningTokens[key] != requestedToken) {
        return existing.then((_) async {
          if (storageKey == key && model.token == requestedToken) {
            await refresh();
          }
        });
      }
      return existing;
    }
    final base = model.baseUrl, token = model.token;
    final done = Completer<void>();
    _running[key] = done.future;
    _runningTokens[key] = token;
    _notify();
    () async {
      try {
        _errors.remove(key);
        if (token.isEmpty) {
          await _resolveGuest(base, key);
        } else {
          await _syncAccount(base, token, key);
        }
      } catch (e) {
        _errors[key] = e is ApiFailure ? e.code : 'network_error';
      } finally {
        _running.remove(key);
        _runningTokens.remove(key);
        done.complete();
        _notify();
      }
    }();
    return done.future;
  }

  bool _current(String base, String token, String key) =>
      model.baseUrl == base && model.token == token && storageKey == key;
  Future<void> _resolveGuest(String base, String key) async {
    final snapshot = _entries(key);
    final targets = snapshot.values.map(asMap).toList();
    for (var i = 0; i < targets.length; i += 100) {
      final batch = targets.skip(i).take(100).toList();
      final result = await _request(
        base,
        '',
        '/public/favorites/resolve',
        body: {
          'items': [
            for (final row in batch)
              {'kind': row['kind'], 'content_id': row['content_id']},
          ],
        },
      );
      if (result['items'] is! List) {
        throw ApiFailure('favoritesUpgradeRequired');
      }
      final saved = _entries(key);
      for (final row in rows(result['items'])) {
        final target = favoriteIdentity(row['kind'], row['content_id']);
        final old = saved[target];
        if (old == null || old['local_id'] != snapshot[target]?['local_id']) {
          continue;
        }
        saved[target] = {...asMap(old), ...row};
      }
      _setEntries(key, saved);
      await _persist(key);
      if (model.baseUrl != base) return;
    }
  }

  Future<void> _syncAccount(String base, String token, String key) async {
    while (_current(base, token, key)) {
      final pending = rows(_state(key)['operations']).take(100).toList();
      if (pending.isEmpty) break;
      final result = await _request(
        base,
        token,
        '/favorites/sync',
        body: {
          'operations': [
            for (final op in pending)
              {
                'operation_id': op['operation_id'],
                'kind': op['kind'],
                'content_id': op['content_id'],
                'favorited': op['favorited'],
              },
          ],
        },
      );
      if (result['acknowledged'] is! List) {
        throw ApiFailure('favoritesUpgradeRequired');
      }
      if (!_current(base, token, key)) return;
      final acknowledged = (result['acknowledged'] as List).toSet();
      if (pending.any((op) => !acknowledged.contains(op['operation_id']))) {
        throw ApiFailure('network_error');
      }
      final guestKey = keyFor(base, 'guest'), guest = _entries(guestKey);
      for (final op in pending) {
        final target = favoriteIdentity(op['kind'], op['content_id']);
        if (op['guest_local_id'] != null &&
            guest[target]?['local_id'] == op['guest_local_id']) {
          guest.remove(target);
        }
      }
      // Persist guest cleanup before acknowledging locally. Server-side operation
      // IDs make retries safe if the app stops between these two writes.
      _setEntries(guestKey, guest);
      await _persist(guestKey);
      _state(key)['operations'] = rows(
        _state(key)['operations'],
      ).where((op) => !acknowledged.contains(op['operation_id'])).toList();
      await _persist(key);
    }
    if (!_current(base, token, key)) return;
    final cloud = <String, dynamic>{};
    for (var page = 1; ; page++) {
      final result = await _request(
        base,
        token,
        '/favorites?page=$page&page_size=100',
      );
      if (result['items'] is! List || result['total'] is! num) {
        throw ApiFailure('favoritesUpgradeRequired');
      }
      if (!_current(base, token, key)) return;
      final incoming = rows(result['items']);
      for (final row in incoming) {
        final target = favoriteIdentity(row['kind'], row['content_id']);
        final old = _entries(key)[target];
        cloud[target] = {
          if (old != null) ...asMap(old),
          ...row,
          'collected_at_ms':
              DateTime.tryParse(
                row['collected_at'] ?? '',
              )?.millisecondsSinceEpoch ??
              DateTime.now().millisecondsSinceEpoch,
        };
      }
      if (page * 100 >= (result['total'] as num)) break;
      if (incoming.isEmpty) throw ApiFailure('network_error');
    }
    // A user may have clicked while the read was in flight. Pending mutations
    // overlay the snapshot so a late response cannot undo the local action.
    final saved = _entries(key);
    for (final op in rows(_state(key)['operations'])) {
      final target = favoriteIdentity(op['kind'], op['content_id']);
      if (op['favorited'] == false) {
        cloud.remove(target);
      } else if (saved[target] != null) {
        cloud[target] = saved[target];
      }
    }
    _setEntries(key, cloud);
    await _persist(key);
    if (rows(_state(key)['operations']).isNotEmpty &&
        _current(base, token, key)) {
      await _syncAccount(base, token, key);
    }
  }
}

class FavoriteButton extends StatelessWidget {
  final bool onDark;
  final bool iconOnly;
  final Json item;
  const FavoriteButton({
    super.key,
    required this.item,
    this.onDark = false,
    this.iconOnly = false,
  });
  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context), kind = item['kind'] as String;
    final selected = m.favorites.contains(kind, (item['id'] as num).toInt());
    if (iconOnly) {
      return IconButton(
        key: ValueKey('favorite-$kind-${item['id']}'),
        tooltip: m.t(selected ? 'favorited' : 'favorite'),
        isSelected: selected,
        color: selected ? industrialBlue : const Color(0xff66768a),
        icon: Icon(selected ? Icons.star_rounded : Icons.star_border_rounded),
        onPressed: () async {
          try {
            await m.favorites.toggle(item);
          } catch (e) {
            if (context.mounted) showError(context, e);
          }
        },
      );
    }
    return TextButton.icon(
      style: onDark
          ? TextButton.styleFrom(foregroundColor: Colors.white)
          : null,
      key: ValueKey('favorite-$kind-${item['id']}'),
      onPressed: () async {
        try {
          await m.favorites.toggle(item);
        } catch (e) {
          if (context.mounted) showError(context, e);
        }
      },
      icon: Icon(selected ? Icons.star_rounded : Icons.star_border_rounded),
      label: Text(m.t(selected ? 'favorited' : 'favorite')),
    );
  }
}

Future<void> confirmFavoriteSignOut(BuildContext context) async {
  final m = ModelScope.of(context), identity = m.scope;
  final yes = await showDialog<bool>(
    context: context,
    builder: (c) => AlertDialog(
      title: Text(m.t('logout')),
      content: Text(m.t('favoriteLogoutConfirm')),
      actions: [
        TextButton(
          onPressed: () => Navigator.pop(c, false),
          child: Text(m.t('cancel')),
        ),
        FilledButton(
          onPressed: () => Navigator.pop(c, true),
          child: Text(m.t('logout')),
        ),
      ],
    ),
  );
  if (yes == true && context.mounted && m.scope == identity) await m.signOut();
}

class FavoritesScreen extends StatefulWidget {
  const FavoritesScreen({super.key});
  @override
  State<FavoritesScreen> createState() => _FavoritesScreenState();
}

class _FavoritesScreenState extends State<FavoritesScreen> {
  String kind = 'vendor';
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted) {
        final store = ModelScope.of(context).favorites;
        store.activate();
        unawaited(store.refresh());
      }
    });
  }

  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context), store = m.favorites;
    final entries = store.entries,
        filtered = entries.where((r) => r['kind'] == kind).toList();
    return Scaffold(
      appBar: appBar(context, m.t('favorites')),
      body: RefreshIndicator(
        onRefresh: store.refresh,
        child: ListView(
          key: PageStorageKey('favorites-${store.storageKey}-$kind'),
          physics: const AlwaysScrollableScrollPhysics(),
          padding: const EdgeInsets.all(20),
          children: [
            Card(
              child: Padding(
                padding: const EdgeInsets.all(16),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      m.token.isEmpty
                          ? m.t('guestFavorites')
                          : m.t('accountFavorites'),
                      style: Theme.of(context).textTheme.titleMedium,
                    ),
                    if (m.token.isNotEmpty)
                      Text(
                        (m.account['phone'] ??
                                m.account['login_name'] ??
                                m.account['username'] ??
                                '')
                            .toString(),
                      ),
                    Text(
                      m.t(
                        m.token.isEmpty
                            ? 'guestFavoritesHelp'
                            : store.pendingCount > 0
                            ? 'favoritesPending'
                            : 'favoritesSynced',
                      ),
                    ),
                    if (m.token.isEmpty)
                      TextButton.icon(
                        icon: const Icon(Icons.cloud_sync_outlined),
                        label: Text(m.t('loginToSync')),
                        onPressed: () =>
                            push(context, const LoginScreen(visitor: true)),
                      ),
                  ],
                ),
              ),
            ),
            if (store.busy) const LinearProgressIndicator(),
            if (store.error.isNotEmpty)
              Card(
                child: Padding(
                  padding: const EdgeInsets.all(12),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(m.t(store.error)),
                      TextButton(
                        onPressed: store.busy ? null : store.refresh,
                        child: Text(m.t('retry')),
                      ),
                    ],
                  ),
                ),
              ),
            SingleChildScrollView(
              scrollDirection: Axis.horizontal,
              child: Row(
                children: [
                  for (final type in ['vendor', 'product'])
                    Padding(
                      padding: const EdgeInsets.only(right: 12),
                      child: ChoiceChip(
                        key: ValueKey('favorites-kind-$type'),
                        selected: kind == type,
                        label: Text(
                          '${m.t(type == 'vendor' ? 'favoriteVendors' : 'favoriteProducts')} (${entries.where((r) => r['kind'] == type).length})',
                        ),
                        onSelected: (_) => setState(() => kind = type),
                      ),
                    ),
                ],
              ),
            ),
            if (filtered.isEmpty) ...[
              message(
                context,
                m.t(
                  kind == 'vendor'
                      ? 'emptyFavoriteVendors'
                      : 'emptyFavoriteProducts',
                ),
              ),
              OutlinedButton(
                onPressed: () => AppNavigation.of(context)?.browse(kind),
                child: Text(
                  m.t(kind == 'vendor' ? 'navBrowse' : 'navProducts'),
                ),
              ),
            ],
            for (final row in filtered)
              if (row['available'] == false || row['item'] == null)
                Card(
                  child: Padding(
                    padding: const EdgeInsets.all(16),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          row['item']?['data']?['name']?.toString() ??
                              '${m.t(kind == 'vendor' ? 'favoriteVendors' : 'favoriteProducts')} #${row['content_id']}',
                        ),
                        Text(m.t('favoriteUnavailable')),
                        TextButton(
                          onPressed: () async {
                            try {
                              await store.remove(row);
                            } catch (e) {
                              if (context.mounted) showError(context, e);
                            }
                          },
                          child: Text(m.t('removeFavorite')),
                        ),
                      ],
                    ),
                  ),
                )
              else
                ItemTile(item: asMap(row['item'])),
          ],
        ),
      ),
    );
  }
}
