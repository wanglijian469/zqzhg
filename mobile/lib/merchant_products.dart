part of 'main.dart';

class MerchantProductsScreen extends StatefulWidget {
  final String status, publication;
  const MerchantProductsScreen({
    super.key,
    this.status = '',
    this.publication = '',
  });
  @override
  State<MerchantProductsScreen> createState() => _MerchantProductsScreenState();
}

class _MerchantProductsScreenState extends State<MerchantProductsScreen> {
  late String status = widget.status, publication = widget.publication;
  String query = '';
  int page = 1, version = 0;
  final search = TextEditingController();
  final scroll = ScrollController();
  Timer? debounce;
  String? owner;
  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    final m = ModelScope.of(context), next = '${m.scope}:${m.token}';
    if (owner != null && owner != next) {
      debounce?.cancel();
      search.clear();
      query = status = publication = '';
      page = 1;
      version++;
      if (scroll.hasClients) scroll.jumpTo(0);
    }
    owner = next;
  }

  @override
  void dispose() {
    debounce?.cancel();
    search.dispose();
    scroll.dispose();
    super.dispose();
  }

  void submit(String text) {
    debounce?.cancel();
    final value = text.trim();
    if (value == query) return;
    setState(() {
      query = value;
      page = 1;
    });
  }

  Future<void> edit(Json? item) async {
    await editMerchant(context, item, 'product');
    if (mounted) setState(() => version++);
  }

  Future<void> action(String action, Json item) async {
    final m = ModelScope.of(context), identity = m.scope;
    if (action == 'history') {
      push(context, HistoryScreen(id: item['id']));
      return;
    }
    if (action == 'preview') {
      push(
        context,
        PreviewScreen(
          data: asMap(item['data']),
          kind: 'product',
          photos: const [],
        ),
      );
      return;
    }
    if (action == 'public') {
      push(context, DetailScreen(kind: 'product', id: item['id']));
      return;
    }
    try {
      if (action == 'share') {
        final r = asMap(await m.api('/public/products/${item['id']}'));
        if (!mounted || m.scope != identity) return;
        final published = asMap(r['item']);
        final vendor = asMap(r['vendor']);
        final showroom = asMap(
          await m.api(
            '/public/showrooms/${Uri.encodeComponent(vendor['showroom_slug'] ?? 'v-${vendor['id']}')}',
          ),
        );
        if (!mounted || m.scope != identity) return;
        final base = showroomShareURL(showroom['canonical_url'], m.baseUrl);
        final url = Uri.parse(
          base,
        ).resolve('/products/${published['id']}').toString();
        await sharePublicText(
          context,
          '${m.content(asMap(published['data']), 'name')}\n$url',
          m.content(asMap(published['data']), 'name'),
          url,
        );
        return;
      }
      final confirmed = await showDialog<bool>(
        context: context,
        builder: (c) => AlertDialog(
          title: Text(m.t('offline')),
          content: Text(m.t('offlineConfirm')),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(c, false),
              child: Text(m.t('cancel')),
            ),
            FilledButton(
              onPressed: () => Navigator.pop(c, true),
              child: Text(m.t('offline')),
            ),
          ],
        ),
      );
      if (confirmed != true || m.scope != identity) return;
      await m.api(
        '/merchant/contents/${item['id']}/offline',
        method: 'POST',
        data: {},
      );
      m.merchant.refresh();
      if (mounted) setState(() => version++);
    } catch (e) {
      if (mounted) showError(context, e);
    }
  }

  Widget product(Json item) {
    final m = ModelScope.of(context), d = asMap(item['data']);
    final spec = (d['parameters'] ?? '')
        .toString()
        .trim()
        .split(RegExp(r'[\r\n]+'))
        .where((s) => s.trim().isNotEmpty)
        .take(2)
        .join(' · ');
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            SizedBox(
              width: 92,
              height: 92,
              child: ClipRRect(
                borderRadius: BorderRadius.circular(10),
                child: Photo(
                  id: imageIDs(d).firstOrNull ?? 0,
                  height: 92,
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
                      fontSize: 16,
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                  if (spec.isNotEmpty)
                    Text(
                      spec,
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(
                        fontSize: 12,
                        color: Color(0xff66768a),
                      ),
                    ),
                  space(6),
                  MerchantStatus(item: item),
                  Wrap(
                    crossAxisAlignment: WrapCrossAlignment.center,
                    children: [
                      TextButton(
                        onPressed: () => edit(item),
                        child: Text(m.t('edit')),
                      ),
                      PopupMenuButton<String>(
                        tooltip: m.t('more'),
                        onSelected: (value) => action(value, item),
                        itemBuilder: (_) => [
                          PopupMenuItem(
                            value: 'preview',
                            child: Text(m.t('draftPreview')),
                          ),
                          PopupMenuItem(
                            value: 'history',
                            child: Text(m.t('history')),
                          ),
                          if (item['public_visible'] == true) ...[
                            PopupMenuItem(
                              value: 'public',
                              child: Text(m.t('view')),
                            ),
                            PopupMenuItem(
                              value: 'share',
                              child: Text(m.t('shareProduct')),
                            ),
                          ],
                          if (item['visible'] == true)
                            PopupMenuItem(
                              value: 'offline',
                              child: Text(m.t('offline')),
                            ),
                        ],
                      ),
                    ],
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }

  Future<void> addProduct() async {
    final m = ModelScope.of(context),
        owner =
            '${ModelScope.of(context).scope}:${ModelScope.of(context).token}';
    try {
      final r = asMap(await m.merchant.vendor());
      if (!mounted || '${m.scope}:${m.token}' != owner) return;
      if (rows(r['items']).isEmpty) {
        push(context, const MerchantManagementScreen());
      } else {
        await edit(null);
      }
    } catch (e) {
      if (mounted) showError(context, e);
    }
  }

  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    final path =
        '/merchant/contents?${Uri(queryParameters: {'kind': 'product', 'q': query, 'status': status, 'public_state': publication, 'page': '$page', 'page_size': '20'}).query}';
    final identity = '${m.scope}:${m.token.hashCode}:$path:$version';
    return MerchantScaffold(
      title: 'productManage',
      actions: [
        if (MediaQuery.textScalerOf(context).scale(1) > 1.3)
          IconButton(
            tooltip: m.t('addProduct'),
            onPressed: addProduct,
            icon: const Icon(Icons.add_box_outlined),
          )
        else
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: 4, vertical: 6),
            child: FilledButton.icon(
              onPressed: addProduct,
              style: FilledButton.styleFrom(
                padding: const EdgeInsets.symmetric(horizontal: 10),
                minimumSize: const Size(44, 44),
              ),
              icon: const Icon(Icons.add, size: 18),
              label: Text(
                m.t('addProduct'),
                style: const TextStyle(fontSize: 14),
              ),
            ),
          ),
      ],
      body: RefreshIndicator(
        onRefresh: () async => setState(() => version++),
        child: ListView(
          controller: scroll,
          padding: const EdgeInsets.all(16),
          physics: const AlwaysScrollableScrollPhysics(),
          children: [
            TextField(
              controller: search,
              decoration: InputDecoration(
                hintText: m.t('productSearch'),
                prefixIcon: const Icon(Icons.search),
                suffixIcon: IconButton(
                  tooltip: m.t('clear'),
                  onPressed: () {
                    search.clear();
                    submit('');
                  },
                  icon: const Icon(Icons.close),
                ),
                isDense: true,
              ),
              onChanged: (v) {
                debounce?.cancel();
                debounce = Timer(
                  const Duration(milliseconds: 350),
                  () => submit(v),
                );
              },
              onSubmitted: submit,
            ),
            space(8),
            SingleChildScrollView(
              scrollDirection: Axis.horizontal,
              child: Row(
                children: [
                  for (final (label, s, p) in [
                    ('productAll', '', ''),
                    ('published', '', 'public'),
                    ('pending', 'pending', ''),
                    ('draft', 'draft', ''),
                    ('needsAttention', 'rejected', ''),
                  ])
                    Padding(
                      padding: const EdgeInsets.only(right: 6),
                      child: ChoiceChip(
                        label: Text(m.t(label)),
                        selected: status == s && publication == p,
                        onSelected: (_) => setState(() {
                          status = s;
                          publication = p;
                          page = 1;
                        }),
                      ),
                    ),
                ],
              ),
            ),
            ExpansionTile(
              minTileHeight: 44,
              tilePadding: EdgeInsets.zero,
              title: Text(
                m.t('moreFilters'),
                style: const TextStyle(fontSize: 14),
              ),
              children: [
                DropdownButtonFormField<String>(
                  initialValue: status,
                  key: ValueKey('review:$status'),
                  isExpanded: true,
                  decoration: InputDecoration(labelText: m.t('reviewStatus')),
                  items: [
                    for (final s in [
                      '',
                      'draft',
                      'pending',
                      'approved',
                      'rejected',
                    ])
                      DropdownMenuItem(
                        value: s,
                        child: Text(m.t(s.isEmpty ? 'allStatus' : s)),
                      ),
                  ],
                  onChanged: (s) => setState(() {
                    status = s ?? '';
                    page = 1;
                  }),
                ),
                space(8),
                DropdownButtonFormField<String>(
                  initialValue: publication,
                  key: ValueKey('publication:$publication'),
                  isExpanded: true,
                  decoration: InputDecoration(labelText: m.t('displayStatus')),
                  items: [
                    for (final s in ['', 'public', 'unpublished'])
                      DropdownMenuItem(
                        value: s,
                        child: Text(
                          m.t(
                            s.isEmpty
                                ? 'allPublic'
                                : s == 'public'
                                ? 'published'
                                : 'unpublished',
                          ),
                        ),
                      ),
                  ],
                  onChanged: (s) => setState(() {
                    publication = s ?? '';
                    page = 1;
                  }),
                ),
              ],
            ),
            AsyncPane(
              key: ValueKey(identity),
              identity: identity,
              load: () => m.api(path),
              builder: (r) {
                final items = rows(r['items']),
                    total = (r['total'] as num).toInt();
                return Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    Text(
                      '${m.t('productTotal')}: $total',
                      style: const TextStyle(fontSize: 12),
                    ),
                    if (items.isEmpty) ...[
                      message(
                        context,
                        m.t(
                          query.isEmpty && status.isEmpty && publication.isEmpty
                              ? 'firstProductHelp'
                              : 'noProductResults',
                        ),
                      ),
                      TextButton(
                        onPressed: () {
                          if (query.isEmpty &&
                              status.isEmpty &&
                              publication.isEmpty) {
                            addProduct();
                          } else {
                            search.clear();
                            setState(() {
                              query = status = publication = '';
                              page = 1;
                            });
                          }
                        },
                        child: Text(
                          m.t(
                            query.isEmpty &&
                                    status.isEmpty &&
                                    publication.isEmpty
                                ? 'addProduct'
                                : 'clear',
                          ),
                        ),
                      ),
                    ],
                    ...items.map(product),
                    Row(
                      children: [
                        Expanded(
                          child: TextButton(
                            onPressed: page > 1
                                ? () => setState(() => page--)
                                : null,
                            child: Text(
                              m.t('previous'),
                              textAlign: TextAlign.center,
                            ),
                          ),
                        ),
                        Text('$page / ${(total / 20).ceil().clamp(1, 100000)}'),
                        Expanded(
                          child: TextButton(
                            onPressed: page * 20 < total
                                ? () => setState(() => page++)
                                : null,
                            child: Text(
                              m.t('next'),
                              textAlign: TextAlign.center,
                            ),
                          ),
                        ),
                      ],
                    ),
                  ],
                );
              },
            ),
            Align(
              alignment: Alignment.centerLeft,
              child: TextButton.icon(
                onPressed: () =>
                    push(context, const RepresentativeSettingsScreen()),
                icon: const Icon(Icons.star_outline, size: 18),
                label: Text(m.t('representativeSettings')),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
