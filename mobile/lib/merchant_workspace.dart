part of 'main.dart';

/// Session-owned futures are shared by My account and the workspace. Failures
/// are evicted so an explicit retry always reaches the server.
class MerchantDataStore {
  final AppModel model;
  MerchantDataStore(this.model);
  String identity = '';
  int version = 0;
  Future<dynamic>? _vendor, _stats;
  void clear() {
    identity = '';
    _vendor = _stats = null;
    version++;
  }

  String get key {
    final next = '${model.scope}:${model.token}';
    if (identity != next) {
      clear();
      identity = next;
    }
    return '${model.scope}:${model.token.hashCode}:$version';
  }

  Future<dynamic> vendor() {
    key;
    if (_vendor != null) return _vendor!;
    final request = model.api('/merchant/contents?kind=vendor&page_size=1');
    _vendor = request;
    unawaited(
      request.then<void>(
        (_) {},
        onError: (Object e, StackTrace s) {
          if (identical(_vendor, request)) _vendor = null;
        },
      ),
    );
    return request;
  }

  Future<dynamic> stats() {
    key;
    if (_stats != null) return _stats!;
    final request = model.api('/merchant/dashboard');
    _stats = request;
    unawaited(
      request.then<void>(
        (_) {},
        onError: (Object e, StackTrace s) {
          if (identical(_stats, request)) _stats = null;
        },
      ),
    );
    return request;
  }

  void refresh() {
    clear();
    model.favoritesChanged();
  }
}

bool isMerchant(AppModel m) =>
    m.token.isNotEmpty && m.account['role'] != 'visitor';
ThemeData merchantTheme(BuildContext context) => Theme.of(context).copyWith(
  colorScheme: Theme.of(
    context,
  ).colorScheme.copyWith(primary: const Color(0xff1769c2)),
  scaffoldBackgroundColor: const Color(0xfff5f7fa),
  textTheme: Theme.of(context).textTheme.apply(
    bodyColor: const Color(0xff182b42),
    displayColor: const Color(0xff182b42),
  ),
  filledButtonTheme: FilledButtonThemeData(
    style: FilledButton.styleFrom(
      backgroundColor: const Color(0xff1769c2),
      minimumSize: const Size(48, 48),
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
    ),
  ),
);

class MerchantScaffold extends StatelessWidget {
  final String title;
  final Widget body;
  final List<Widget> actions;
  const MerchantScaffold({
    super.key,
    required this.title,
    required this.body,
    this.actions = const [],
  });
  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    if (!isMerchant(m)) return const MerchantScreen();
    return Theme(
      data: merchantTheme(context),
      child: Scaffold(
        appBar: appBar(context, m.t(title), extraActions: actions),
        body: SafeArea(
          top: false,
          child: KeyedSubtree(
            key: ValueKey('${m.scope}:${m.token.hashCode}'),
            child: body,
          ),
        ),
      ),
    );
  }
}

class MerchantVendorPane extends StatelessWidget {
  final Widget Function(Json? item) builder;
  const MerchantVendorPane({super.key, required this.builder});
  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context), key = m.merchant.key;
    return AsyncPane(
      key: ValueKey(key),
      identity: key,
      load: m.merchant.vendor,
      builder: (r) {
        final items = rows(r['items']);
        return builder(items.isEmpty ? null : items.first);
      },
    );
  }
}

class MerchantStatus extends StatelessWidget {
  final Json item;
  const MerchantStatus({super.key, required this.item});
  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    final note = (item['review_note'] ?? '').toString().replaceFirst(
      RegExp(r' SHA256:[a-f0-9]{64}$'),
      '',
    );
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Wrap(
          spacing: 6,
          runSpacing: 6,
          children: [
            for (final label in [
              m.t(
                item['review_note'] == 'offline'
                    ? 'reviewStopped'
                    : item['status'] ?? 'draft',
              ),
              m.t(item['public_visible'] == true ? 'published' : 'unpublished'),
            ])
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
                decoration: BoxDecoration(
                  color: const Color(0xffeef4fb),
                  borderRadius: BorderRadius.circular(6),
                ),
                child: Text(label, style: const TextStyle(fontSize: 12)),
              ),
          ],
        ),
        if (item['status'] == 'rejected' &&
            note.isNotEmpty &&
            note != 'offline')
          Padding(
            padding: const EdgeInsets.only(top: 6),
            child: Text(
              note,
              style: const TextStyle(color: Color(0xffa2392d), fontSize: 13),
            ),
          ),
        if (note == 'offline')
          Text(m.t('offlineRecord'), style: const TextStyle(fontSize: 12)),
      ],
    );
  }
}

class MerchantCompanyCard extends StatelessWidget {
  final Json item;
  const MerchantCompanyCard({super.key, required this.item});
  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context),
        d = asMap(item['data']),
        p = asMap(d['company_profile'] ?? {});
    final logo = p['logo_media_id'];
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            SizedBox(
              width: 56,
              height: 56,
              child: ClipRRect(
                borderRadius: BorderRadius.circular(12),
                child: Photo(
                  id: logo is num ? logo.toInt() : 0,
                  height: 56,
                  fit: BoxFit.contain,
                ),
              ),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    m.content(d, 'name'),
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(
                      fontSize: 18,
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                  if ((d['region_name'] ?? '').toString().isNotEmpty)
                    Text(
                      d['region_name'],
                      style: const TextStyle(fontSize: 13),
                    ),
                  space(6),
                  MerchantStatus(item: item),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}

Future<void> editMerchant(
  BuildContext context,
  Json? item,
  String kind, {
  String? section,
}) async {
  final m = ModelScope.of(context), identity = m.scope;
  await Navigator.push(
    context,
    MaterialPageRoute(
      builder: (_) =>
          EditorScreen(kind: kind, item: item, initialSection: section),
    ),
  );
  if (m.scope == identity) m.merchant.refresh();
}

Widget noMerchantCompany(BuildContext context) {
  final m = ModelScope.of(context);
  return Column(
    crossAxisAlignment: CrossAxisAlignment.stretch,
    children: [
      message(context, m.t('noBooth')),
      FilledButton.icon(
        onPressed: () => editMerchant(context, null, 'vendor'),
        icon: const Icon(Icons.add_business_outlined),
        label: Text(m.t('freeEntry')),
      ),
    ],
  );
}

class MerchantWorkspace extends StatelessWidget {
  const MerchantWorkspace({super.key});
  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    return MerchantScaffold(
      title: 'merchantCenter',
      actions: [
        IconButton(
          tooltip: m.t('appSettings'),
          onPressed: () => push(context, const AccountSettingsScreen()),
          icon: const Icon(Icons.settings_outlined),
        ),
      ],
      body: RefreshIndicator(
        onRefresh: () async {
          m.merchant.refresh();
          try {
            await Future.wait([m.merchant.vendor(), m.merchant.stats()]);
          } catch (e) {
            if (context.mounted) showError(context, e);
          }
        },
        child: ListView(
          padding: const EdgeInsets.all(16),
          physics: const AlwaysScrollableScrollPhysics(),
          children: [
            MerchantVendorPane(
              builder: (item) => item == null
                  ? noMerchantCompany(context)
                  : Column(
                      crossAxisAlignment: CrossAxisAlignment.stretch,
                      children: [
                        MerchantCompanyCard(item: item),
                        _MerchantCompleteness(item: item),
                        space(12),
                        LayoutBuilder(
                          builder: (context, constraints) {
                            final width = constraints.maxWidth,
                                scale = MediaQuery.textScalerOf(
                                  context,
                                ).scale(1);
                            final columns = scale >= 1.7 ? 1 : 2;
                            final actions = <(String, IconData, VoidCallback)>[
                              (
                                'editShowroom',
                                Icons.edit_outlined,
                                () => editMerchant(context, item, 'vendor'),
                              ),
                              (
                                'addProduct',
                                Icons.add_box_outlined,
                                () => editMerchant(context, null, 'product'),
                              ),
                              (
                                'productManage',
                                Icons.inventory_2_outlined,
                                () => push(
                                  context,
                                  const MerchantProductsScreen(),
                                ),
                              ),
                              (
                                'shareShowroom',
                                Icons.share_outlined,
                                () => openMerchantPromotion(context, item),
                              ),
                            ];
                            return Wrap(
                              spacing: 8,
                              runSpacing: 8,
                              children: [
                                for (final (label, icon, action) in actions)
                                  SizedBox(
                                    width:
                                        (width - (columns - 1) * 8) / columns,
                                    child: OutlinedButton.icon(
                                      onPressed: action,
                                      icon: Icon(icon, size: 20),
                                      label: Text(
                                        m.t(label),
                                        textAlign: TextAlign.center,
                                      ),
                                    ),
                                  ),
                              ],
                            );
                          },
                        ),
                        ListTile(
                          contentPadding: EdgeInsets.zero,
                          leading: const Icon(Icons.storefront_outlined),
                          title: Text(m.t('boothManage')),
                          trailing: const Icon(Icons.chevron_right),
                          onTap: () =>
                              push(context, const MerchantManagementScreen()),
                        ),
                      ],
                    ),
            ),
            space(12),
            AsyncPane(
              key: ValueKey('stats:${m.merchant.key}'),
              identity: m.merchant.key,
              load: m.merchant.stats,
              builder: (r) {
                final counts = asMap(r['products']);
                return Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    Card(
                      child: Padding(
                        padding: const EdgeInsets.all(12),
                        child: Row(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            for (final (key, label) in [
                              ('public', 'published'),
                              ('pending', 'pending'),
                              ('draft', 'draft'),
                            ])
                              Expanded(
                                child: InkWell(
                                  onTap: () => push(
                                    context,
                                    MerchantProductsScreen(
                                      status: key == 'public' ? '' : key,
                                      publication: key == 'public'
                                          ? 'public'
                                          : '',
                                    ),
                                  ),
                                  child: Column(
                                    children: [
                                      Text(
                                        '${counts[key] ?? 0}',
                                        style: const TextStyle(
                                          fontSize: 24,
                                          fontWeight: FontWeight.bold,
                                        ),
                                      ),
                                      Text(
                                        m.t(label),
                                        textAlign: TextAlign.center,
                                        style: const TextStyle(fontSize: 13),
                                      ),
                                    ],
                                  ),
                                ),
                              ),
                          ],
                        ),
                      ),
                    ),
                    if ((counts['rejected'] as num? ?? 0) > 0)
                      ListTile(
                        contentPadding: EdgeInsets.zero,
                        leading: const Icon(
                          Icons.error_outline,
                          color: Color(0xffa2392d),
                        ),
                        title: Text(
                          '${m.t('rejectedProductsReminder')} (${counts['rejected']})',
                        ),
                        trailing: const Icon(Icons.chevron_right),
                        onTap: () => push(
                          context,
                          const MerchantProductsScreen(status: 'rejected'),
                        ),
                      ),
                  ],
                );
              },
            ),
          ],
        ),
      ),
    );
  }
}

class _MerchantCompleteness extends StatelessWidget {
  final Json item;
  const _MerchantCompleteness({required this.item});
  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context),
        d = asMap(item['data']),
        p = asMap(d['company_profile'] ?? {});
    final required = validateFormData(d, 'vendor').keys.toList();
    final suggestions = <(String, String)>[
      if ((p['summary'] ?? '').toString().trim().isEmpty &&
          (d['description'] ?? '').toString().trim().isEmpty)
        ('summary', 'basic'),
      if ((p['logo_media_id'] as num? ?? 0) == 0) ('logo', 'brand'),
      if ((p['cover_media_id'] as num? ?? 0) == 0) ('cover', 'brand'),
      if (imageIDs(d).isEmpty) ('boothGallery', 'gallery'),
      if ((d['address'] ?? '').toString().trim().isEmpty)
        ('address', 'contact'),
      if (rows(p['contacts']).isEmpty) ('regionalContacts', 'profile'),
    ];
    if (required.isEmpty && suggestions.isEmpty) return const SizedBox();
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              m.t(required.isEmpty ? 'suggestedDetails' : 'requiredDetails'),
            ),
            Wrap(
              spacing: 8,
              children: [
                for (final field in required)
                  TextButton(
                    onPressed: () =>
                        editMerchant(context, item, 'vendor', section: 'basic'),
                    child: Text('${formRules[field]?[m.language] ?? field}'),
                  ),
                for (final (label, section) in suggestions)
                  TextButton(
                    onPressed: () =>
                        editMerchant(context, item, 'vendor', section: section),
                    child: Text(m.t(label)),
                  ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

void openMerchantPromotion(BuildContext context, Json item) {
  if (item['public_visible'] != true) {
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text(ModelScope.of(context).t('publicOnlyShare'))),
    );
    return;
  }
  push(
    context,
    ShowroomPromotion(slug: item['showroom_slug'] ?? 'v-${item['id']}'),
  );
}

class MerchantManagementScreen extends StatelessWidget {
  const MerchantManagementScreen({super.key});
  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    return MerchantScaffold(
      title: 'boothManage',
      body: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          MerchantVendorPane(
            builder: (item) {
              if (item == null) return noMerchantCompany(context);
              Widget group(
                String label,
                List<(String, IconData, VoidCallback)> tasks,
              ) => Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  Padding(
                    padding: const EdgeInsets.only(top: 16, bottom: 6),
                    child: Text(
                      m.t(label),
                      style: const TextStyle(
                        fontSize: 17,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                  ),
                  Card(
                    child: Column(
                      children: [
                        for (final (key, icon, action) in tasks)
                          ListTile(
                            visualDensity: VisualDensity.compact,
                            minVerticalPadding: 10,
                            leading: Icon(icon, color: const Color(0xff1769c2)),
                            title: Text(
                              m.t(key),
                              style: const TextStyle(fontSize: 15),
                            ),
                            trailing: const Icon(Icons.chevron_right),
                            onTap: action,
                          ),
                      ],
                    ),
                  ),
                ],
              );
              VoidCallback edit(String section) =>
                  () => editMerchant(context, item, 'vendor', section: section);
              return Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  MerchantCompanyCard(item: item),
                  if (item['status'] == 'pending')
                    message(context, m.t('boothPendingReadOnly')),
                  group('companyMaterials', [
                    ('boothBasics', Icons.business_outlined, edit('basic')),
                    ('boothProfile', Icons.factory_outlined, edit('profile')),
                    ('contactInfo', Icons.phone_outlined, edit('contact')),
                  ]),
                  group('displayMaterials', [
                    ('boothBrand', Icons.image_outlined, edit('brand')),
                    (
                      'boothGallery',
                      Icons.photo_library_outlined,
                      edit('gallery'),
                    ),
                    (
                      'shortVideos',
                      Icons.video_library_outlined,
                      () => push(
                        context,
                        const MerchantResourcesScreen(kind: 'video'),
                      ),
                    ),
                    (
                      'electronicCatalogs',
                      Icons.menu_book_outlined,
                      () => push(
                        context,
                        const MerchantResourcesScreen(kind: 'catalog'),
                      ),
                    ),
                    (
                      'boothCredentials',
                      Icons.workspace_premium_outlined,
                      edit('credentials'),
                    ),
                  ]),
                  group('viewAndReview', [
                    (
                      'previewShowroom',
                      Icons.visibility_outlined,
                      () => push(
                        context,
                        PreviewScreen(
                          data: asMap(item['data']),
                          kind: 'vendor',
                          photos: const [],
                        ),
                      ),
                    ),
                    (
                      'history',
                      Icons.history,
                      () => push(context, HistoryScreen(id: item['id'])),
                    ),
                  ]),
                ],
              );
            },
          ),
        ],
      ),
    );
  }
}

Future<void> sharePublicText(
  BuildContext context,
  String text,
  String title,
  String url,
) async {
  try {
    await sharingBridge.invokeMethod('shareText', {
      'text': text,
      'title': title,
    });
  } catch (_) {
    if (!context.mounted) return;
    final m = ModelScope.of(context);
    await showDialog<void>(
      context: context,
      builder: (c) => AlertDialog(
        title: Text(m.t('copyFallback')),
        content: SelectableText(url),
        actions: [
          TextButton(
            onPressed: () async {
              await Clipboard.setData(ClipboardData(text: url));
              if (c.mounted) Navigator.pop(c);
            },
            child: Text(m.t('copyLink')),
          ),
        ],
      ),
    );
  }
}
