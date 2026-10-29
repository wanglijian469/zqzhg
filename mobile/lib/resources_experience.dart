part of 'main.dart';

String resourceTitle(AppModel m, Json r) =>
    m.language == 'en' && (r['data']['title_en'] ?? '').toString().isNotEmpty
    ? r['data']['title_en']
    : r['data']['title'];

class CompanyShowroomScreen extends StatefulWidget {
  final String slug;
  final int initialTab;
  const CompanyShowroomScreen({
    super.key,
    required this.slug,
    this.initialTab = 5,
  });
  @override
  State<CompanyShowroomScreen> createState() => _CompanyShowroomScreenState();
}

class _CompanyShowroomScreenState extends State<CompanyShowroomScreen> {
  late int tab = widget.initialTab;
  late final visited = <int>{widget.initialTab};
  Widget? stableHero;
  Object? heroData;
  String? heroScope;
  final productState = <String, dynamic>{};
  final tabKeys = {
    for (final i in [5, 0, 1, 2, 3, 4]) i: GlobalKey(),
  };
  bool positioned = false;
  void choose(int value) {
    FocusScope.of(context).unfocus();
    if (value == tab) return;
    setState(() {
      tab = value;
      visited.add(value);
    });
    WidgetsBinding.instance.addPostFrameCallback((_) {
      final c = tabKeys[value]?.currentContext;
      if (c != null) {
        Scrollable.of(c, axis: Axis.horizontal).position.ensureVisible(
          c.findRenderObject()!,
          alignment: .5,
          duration: const Duration(milliseconds: 180),
        );
      }
    });
  }

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    unawaited(ModelScope.of(context).loadHalls());
  }

  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    return Scaffold(
      appBar: appBar(context, m.t('brand')),
      body: AsyncPane(
        identity: '${m.baseUrl}:showroom:${widget.slug}',
        load: () =>
            m.api('/public/showrooms/${Uri.encodeComponent(widget.slug)}'),
        builder: (result) {
          final item = asMap(result['item']),
              data = asMap(item['data']),
              profile = asMap(data['company_profile'] ?? {});
          final resourceCounts = asMap(result['resource_counts'] ?? {});
          final visibleTabs = [
            5,
            0,
            if ((result['product_count'] as num? ?? 0) > 0) 1,
            if ((resourceCounts['video'] as num? ?? 0) > 0) 2,
            if ((resourceCounts['catalog'] as num? ?? 0) > 0) 3,
            4,
          ];
          if (!visibleTabs.contains(tab)) {
            tab = 5;
            visited.add(5);
          }
          final ownHall = m.availableHalls
              .where((h) => h.id == (item['hall_id'] ?? 1))
              .firstOrNull;
          if (!positioned) {
            positioned = true;
            WidgetsBinding.instance.addPostFrameCallback((_) {
              final c = tabKeys[tab]?.currentContext;
              if (c != null) {
                Scrollable.of(
                  c,
                  axis: Axis.horizontal,
                ).position.ensureVisible(c.findRenderObject()!, alignment: .5);
              }
            });
          }
          void returnHall() {
            if (ownHall == null) return;
            m.selectHall(ownHall);
            if (m.openHome != null) {
              Navigator.of(context).popUntil((r) => r.isFirst);
              m.openHome!();
            } else {
              push(context, const HomeScreen());
            }
          }

          final nextHeroScope = '${m.baseUrl}:${widget.slug}:${m.language}';
          if (heroScope != nextHeroScope || !identical(heroData, result)) {
            heroScope = nextHeroScope;
            heroData = result;
            stableHero = NativeCompanyHero(
              item: item,
              slug: widget.slug,
              onProducts: () => choose(1),
              showProducts: visibleTabs.contains(1),
              onContact: () => choose(4),
            );
          }
          final phone = (data['phone'] ?? '').toString(),
              wechat = (data['wechat'] ?? '').toString();
          return AnalyticsScreen(
            page: '/v/${widget.slug}',
            kind: 'vendor',
            id: item['id'],
            child: Scaffold(
              bottomNavigationBar: Material(
                color: Colors.white,
                elevation: 4,
                child: SafeArea(
                  top: false,
                  child: Row(
                    children: [
                      if (visibleTabs.contains(1))
                        Expanded(
                          child: TextButton.icon(
                            onPressed: () => choose(1),
                            icon: const Icon(Icons.inventory_2_outlined),
                            label: Text(m.t('navProducts')),
                          ),
                        ),
                      if (phone.isNotEmpty || wechat.isNotEmpty)
                        Expanded(
                          child: TextButton.icon(
                            onPressed: () => phone.isNotEmpty
                                ? dialContact(context, phone)
                                : copyNativeText(context, wechat),
                            icon: Icon(
                              phone.isNotEmpty
                                  ? Icons.phone_outlined
                                  : Icons.chat_outlined,
                            ),
                            label: Text(
                              m.t(phone.isNotEmpty ? 'phone' : 'wechat'),
                            ),
                          ),
                        ),
                      Expanded(
                        child: TextButton(
                          onPressed: () => choose(4),
                          child: Text(m.t('showroomContact')),
                        ),
                      ),
                    ],
                  ),
                ),
              ),
              body: CustomScrollView(
                key: PageStorageKey('showroom:${m.baseUrl}:${widget.slug}'),
                slivers: [
                  SliverToBoxAdapter(
                    child: Align(
                      alignment: Alignment.centerLeft,
                      child: TextButton.icon(
                        onPressed: ownHall == null ? null : returnHall,
                        icon: const Icon(Icons.arrow_back, size: 16),
                        label: Text(
                          m.t('backExhibition') +
                              (ownHall == null
                                  ? ''
                                  : ' · ${exhibitionHallName(m, ownHall)}'),
                        ),
                      ),
                    ),
                  ),
                  SliverToBoxAdapter(child: stableHero!),
                  SliverPersistentHeader(
                    pinned: true,
                    delegate: ShowroomTabsHeader(
                      height:
                          56 *
                          max(
                            1,
                            MediaQuery.textScalerOf(context).scale(12) / 12,
                          ),
                      child: SingleChildScrollView(
                        scrollDirection: Axis.horizontal,
                        child: Row(
                          children: [
                            for (final i in visibleTabs)
                              Padding(
                                key: tabKeys[i],
                                padding: const EdgeInsets.symmetric(
                                  horizontal: 4,
                                ),
                                child: ChoiceChip(
                                  label: Text(
                                    m.t(
                                      [
                                        'companyIntro',
                                        'exhibits',
                                        'shortVideos',
                                        'electronicCatalogs',
                                        'showroomContact',
                                        'portalHome',
                                      ][i],
                                    ),
                                  ),
                                  selected: tab == i,
                                  onSelected: (_) => choose(i),
                                ),
                              ),
                          ],
                        ),
                      ),
                    ),
                  ),
                  SliverPadding(
                    padding: const EdgeInsets.all(16),
                    sliver: SliverToBoxAdapter(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.stretch,
                        children: [
                          if (visited.contains(5))
                            Offstage(
                              key: const ValueKey('showroom-home'),
                              offstage: tab != 5,
                              child: ShowroomHomeBody(
                                data: data,
                                result: asMap(result),
                                onTab: choose,
                              ),
                            ),
                          if (tab == 0) ...[
                            CompanyPortalBody(
                              data: {...data, 'media_ids': <int>[]},
                              result: asMap(result),
                            ),
                            if (companyGallery(data).isNotEmpty) ...[
                              space(),
                              Text(
                                m.t('showroomGallery'),
                                style: Theme.of(context).textTheme.titleLarge,
                              ),
                              NativeCompanyGallery(items: companyGallery(data)),
                            ],
                          ],
                          if (visited.contains(1))
                            Offstage(
                              key: const ValueKey('showroom-products'),
                              offstage: tab != 1,
                              child: NativeCompanyProducts(
                                vendorID: item['id'],
                                hall: ownHall,
                                categories: rows(result['categories']),
                                memory: productState,
                              ),
                            ),
                          for (final resourceTab in [2, 3])
                            if (visited.contains(resourceTab))
                              Offstage(
                                key: ValueKey('showroom-resource-$resourceTab'),
                                offstage: tab != resourceTab,
                                child: NativeResourceList(
                                  key: ValueKey('${widget.slug}:$resourceTab'),
                                  vendorID: item['id'],
                                  kind: resourceTab == 2 ? 'video' : 'catalog',
                                ),
                              ),

                          if (tab == 4) ...[
                            Text(
                              m.t('showroomContact'),
                              style: Theme.of(context).textTheme.titleLarge,
                            ),
                            space(12),
                            Text(
                              m.content(data, 'name'),
                              style: Theme.of(context).textTheme.titleMedium,
                            ),
                            if (m.content(profile, 'contact_name').isNotEmpty)
                              ListTile(
                                contentPadding: EdgeInsets.zero,
                                title: Text(m.t('contactName')),
                                subtitle: Text(
                                  m.content(profile, 'contact_name'),
                                ),
                              ),
                            Contact(data: data),
                            if ((data['address'] ?? '').toString().isNotEmpty)
                              ListTile(
                                contentPadding: EdgeInsets.zero,
                                title: Text(m.t('companyAddress')),
                                subtitle: Text(data['address']),
                              ),
                            if ((profile['website'] ?? '')
                                .toString()
                                .isNotEmpty)
                              TextButton(
                                onPressed: () => launchUrl(
                                  Uri.parse(profile['website']),
                                  mode: LaunchMode.externalApplication,
                                ),
                                child: Text(profile['website']),
                              ),
                            for (final c in rows(profile['contacts']))
                              if ((c['name'] ?? c['phone'] ?? c['wechat'] ?? '')
                                  .toString()
                                  .isNotEmpty)
                                Card(
                                  child: Padding(
                                    padding: const EdgeInsets.all(14),
                                    child: Column(
                                      crossAxisAlignment:
                                          CrossAxisAlignment.stretch,
                                      children: [
                                        Text(
                                          [
                                                m.content(c, 'area'),
                                                m.content(c, 'name'),
                                              ]
                                              .where((v) => v.isNotEmpty)
                                              .join(' · '),
                                        ),
                                        Contact(data: c),
                                      ],
                                    ),
                                  ),
                                ),
                          ],
                          space(16),
                          const ICPRegistrationTile(),
                        ],
                      ),
                    ),
                  ),
                ],
              ),
            ),
          );
        },
      ),
    );
  }
}

class NativeCompanyProducts extends StatefulWidget {
  final int vendorID;
  final ExhibitionHall? hall;
  final List<Json> categories;
  final Json memory;
  const NativeCompanyProducts({
    super.key,
    required this.vendorID,
    required this.hall,
    required this.categories,
    required this.memory,
  });
  @override
  State<NativeCompanyProducts> createState() => _NativeCompanyProductsState();
}

class _NativeCompanyProductsState extends State<NativeCompanyProducts> {
  late final TextEditingController search;
  late int page, category;
  late String term, order;
  @override
  void initState() {
    super.initState();
    search = TextEditingController(text: widget.memory['query'] ?? '');
    term = widget.memory['term'] ?? '';
    page = widget.memory['page'] ?? 1;
    category = widget.memory['category'] ?? 0;
    order = widget.memory['order'] ?? '';
  }

  void change(VoidCallback fn) {
    setState(fn);
    widget.memory.addAll({
      'query': search.text,
      'term': term,
      'page': page,
      'category': category,
      'order': order,
    });
  }

  void submit() {
    FocusScope.of(context).unfocus();
    change(() {
      term = search.text.trim();
      page = 1;
    });
  }

  @override
  void dispose() {
    search.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    final params = Uri(
      queryParameters: {
        'vendor_id': widget.vendorID.toString(),
        'page': '$page',
        'page_size': '24',
        'q': term,
        if (category > 0) 'category_id': '$category',
        if (m.modernPublicApi && order.isNotEmpty) 'sort': order,
      },
    ).query;
    final path = m.publicPath(
      '/public/products?$params',
      hall: widget.hall ?? ExhibitionHall.agriculture,
    );
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        NativeFieldLabel(
          label: m.t('showroomKeyword'),
          child: TextField(
            controller: search,
            textInputAction: TextInputAction.search,
            decoration: InputDecoration(
              suffixIcon: IconButton(
                tooltip: m.t('search'),
                onPressed: submit,
                icon: const Icon(Icons.search),
              ),
            ),
            onSubmitted: (_) => submit(),
          ),
        ),
        NativeFilterPanel(
          count: (category > 0 ? 1 : 0) + (order.isEmpty ? 0 : 1),
          panelKey: const PageStorageKey('company-product-filters'),
          children: [
            NativeFieldLabel(
              label: m.t('category'),
              child: DropdownButtonFormField<int>(
                key: ValueKey(category),
                initialValue: category,
                isExpanded: true,
                decoration: InputDecoration(),
                items: [
                  DropdownMenuItem(value: 0, child: Text(m.t('allCategories'))),
                  for (final c in widget.categories)
                    DropdownMenuItem(
                      value: c['id'],
                      child: Text(
                        m.content(c, 'name'),
                        overflow: TextOverflow.ellipsis,
                      ),
                    ),
                ],
                onChanged: (v) => change(() {
                  category = v ?? 0;
                  page = 1;
                }),
              ),
            ),
            if (m.modernPublicApi)
              NativeFieldLabel(
                label: m.t('sort'),
                child: DropdownButtonFormField<String>(
                  initialValue: order,
                  isExpanded: true,
                  decoration: InputDecoration(),
                  items: [
                    DropdownMenuItem(
                      value: '',
                      child: Text(m.t('defaultSort')),
                    ),
                    DropdownMenuItem(
                      value: 'latest',
                      child: Text(m.t('latestSort')),
                    ),
                  ],
                  onChanged: (v) => change(() {
                    order = v ?? '';
                    page = 1;
                  }),
                ),
              ),
            if (category > 0 || term.isNotEmpty || order.isNotEmpty)
              TextButton(
                onPressed: () => change(() {
                  category = 0;
                  term = '';
                  order = '';
                  search.clear();
                  page = 1;
                }),
                child: Text(m.t('clear')),
              ),
          ],
        ),
        AsyncPane(
          identity: m.baseUrl + path,
          load: () => m.api(path),
          builder: (r) {
            final products = uniqueContent(rows(r['items'])).map((p) {
              final d = asMap(p['data']);
              final c = widget.categories
                  .where((c) => c['id'] == d['category_id'])
                  .firstOrNull;
              return <String, dynamic>{
                ...p,
                'data': {
                  ...d,
                  if (c != null) 'category_name': m.content(c, 'name'),
                },
              };
            }).toList();
            return Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                Text(
                  '${m.t('productResults')}: ${r['total']}',
                  style: Theme.of(context).textTheme.bodySmall,
                ),
                if (products.isEmpty)
                  message(
                    context,
                    m.t(
                      term.isNotEmpty || category > 0
                          ? 'showroomNoMatches'
                          : 'showroomEmpty',
                    ),
                  ),
                ExhibitGrid(items: products, showVendor: false),
                NativePagination(
                  page: page,
                  total: (r['total'] as num).toInt(),
                  size: (r['page_size'] as num?)?.toInt() ?? 24,
                  onPage: (n) => change(() => page = n),
                ),
              ],
            );
          },
        ),
      ],
    );
  }
}

class NativeResourceList extends StatefulWidget {
  final int? vendorID, productID;
  final String? kind;
  final bool management;
  const NativeResourceList({
    super.key,
    this.vendorID,
    this.productID,
    this.kind,
    this.management = false,
  });
  @override
  State<NativeResourceList> createState() => _NativeResourceListState();
}

class _NativeResourceListState extends State<NativeResourceList> {
  int page = 1, version = 0;
  String purpose = '', status = '', query = '';
  Timer? pollTimer;
  @override
  void initState() {
    super.initState();
    if (widget.management) {
      pollTimer = Timer.periodic(const Duration(seconds: 3), (_) {
        if (mounted) setState(() => version++);
      });
    }
  }

  @override
  void dispose() {
    pollTimer?.cancel();
    super.dispose();
  }

  Future<void> refresh() async {
    if (mounted) setState(() => version++);
  }

  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    final q = {
      'page': '$page',
      'page_size': '12',
      if (widget.vendorID != null) 'vendor_id': '${widget.vendorID}',
      if (widget.productID != null) 'product_id': '${widget.productID}',
      if (widget.kind != null) 'kind': widget.kind!,
      'purpose': purpose,
      'status': status,
      'q': query,
    };
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        if (widget.management) ...[
          FilledButton.icon(
            onPressed: () async {
              await Navigator.push(
                context,
                MaterialPageRoute(
                  builder: (_) => NativeResourceEditor(
                    kind: widget.kind!,
                    productID: widget.productID,
                  ),
                ),
              );
              refresh();
            },
            icon: const Icon(Icons.add),
            label: Text(m.t('newResource')),
          ),
          space(),
          NativeFieldLabel(
            label: m.t('keyword'),
            child: TextField(
              decoration: InputDecoration(),
              onChanged: (v) => setState(() {
                query = v;
                page = 1;
              }),
            ),
          ),
          space(),
          NativeFieldLabel(
            label: m.t('reviewStatus'),
            child: DropdownButtonFormField<String>(
              initialValue: status,
              isExpanded: true,
              decoration: InputDecoration(),
              items: [
                DropdownMenuItem(value: '', child: Text(m.t('allStatus'))),
                for (final s in ['draft', 'pending', 'approved', 'rejected'])
                  DropdownMenuItem(value: s, child: Text(m.t(s))),
              ],
              onChanged: (v) => setState(() {
                status = v ?? '';
                page = 1;
              }),
            ),
          ),
        ],
        if (!widget.management && widget.kind == 'video')
          NativeFieldLabel(
            label: m.t('purpose'),
            child: DropdownButtonFormField<String>(
              initialValue: purpose,
              isExpanded: true,
              decoration: InputDecoration(),
              items: [
                DropdownMenuItem(value: '', child: Text(m.t('allVideoTypes'))),
                for (final p in [
                  ['company', 'companyVideo'],
                  ['production', 'productionVideo'],
                  ['product', 'productVideo'],
                ])
                  DropdownMenuItem(value: p[0], child: Text(m.t(p[1]))),
              ],
              onChanged: (v) => setState(() {
                purpose = v ?? '';
                page = 1;
              }),
            ),
          ),
        space(),
        AsyncPane(
          identity:
              '${m.scope}:${widget.management}:$version:${Uri(queryParameters: q).query}',
          load: () => m.api(
            '/${widget.management ? 'merchant' : 'public'}/resources?${Uri(queryParameters: q).query}',
          ),
          builder: (r) => Column(
            children: [
              if (rows(r['items']).isEmpty)
                message(context, m.t('noResources')),
              for (final item in rows(r['items']).where(
                (e) =>
                    widget.management ||
                    widget.vendorID == null ||
                    e['vendor_id'] == widget.vendorID,
              ))
                if (!widget.management)
                  NativePublicResourceCard(item: item)
                else
                  Card(
                    child: Padding(
                      padding: const EdgeInsets.all(14),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          if (!widget.management)
                            InkWell(
                              onTap: () => push(
                                context,
                                NativeResourceViewer(item: item),
                              ),
                              child: item['data']['cover_media_id'] != 0
                                  ? Photo(
                                      id: item['data']['cover_media_id'],
                                      aspectRatio: 16 / 9,
                                    )
                                  : item['kind'] == 'video'
                                  ? Photo(
                                      id: item['data']['file_media_id'],
                                      height: 180,
                                    )
                                  : const SizedBox(
                                      height: 100,
                                      child: Center(
                                        child: Icon(
                                          Icons.picture_as_pdf,
                                          size: 44,
                                        ),
                                      ),
                                    ),
                            ),
                          Text(
                            resourceTitle(m, item),
                            style: const TextStyle(
                              fontSize: 19,
                              fontWeight: FontWeight.bold,
                            ),
                          ),
                          if (item['kind'] == 'video')
                            Text(
                              m.t(
                                    item['data']['purpose'] == 'production'
                                        ? 'productionVideo'
                                        : item['data']['purpose'] == 'product'
                                        ? 'productVideo'
                                        : 'companyVideo',
                                  ) +
                                  ((item['file']?['duration'] as num? ?? 0) > 0
                                      ? ' · ${(item['file']['duration'] as num).ceil()} s'
                                      : ''),
                            )
                          else if ((item['file']?['page_count'] ??
                                  (item['data']['page_media_ids'] as List? ??
                                          [])
                                      .length) >
                              0)
                            Text(
                              '${item['file']?['page_count'] ?? (item['data']['page_media_ids'] as List).length} ${m.t('pageNumber')}',
                            ),
                          if (widget.management) ...[
                            Wrap(
                              spacing: 8,
                              children: [
                                Chip(label: Text(m.t(item['status']))),
                                Chip(
                                  label: Text(
                                    m.t(
                                      item['public_visible'] == true
                                          ? 'published'
                                          : 'unpublished',
                                    ),
                                  ),
                                ),
                                if (item['file'] != null)
                                  Chip(label: Text(m.t(item['file']['state']))),
                              ],
                            ),
                            if ((item['review_note'] ?? '')
                                .toString()
                                .isNotEmpty)
                              message(
                                context,
                                item['review_note'] == 'offline'
                                    ? m.t('offline')
                                    : item['review_note'],
                              ),
                          ],
                          Wrap(
                            spacing: 12,
                            runSpacing: 8,
                            children: [
                              OutlinedButton(
                                onPressed: () => push(
                                  context,
                                  NativeResourceViewer(
                                    item: item,
                                    private: widget.management,
                                  ),
                                ),
                                child: Text(
                                  m.t(
                                    widget.management
                                        ? 'preview'
                                        : item['kind'] == 'video'
                                        ? 'play'
                                        : 'readCatalog',
                                  ),
                                ),
                              ),
                              if (widget.management) ...[
                                OutlinedButton(
                                  onPressed: () async {
                                    await Navigator.push(
                                      context,
                                      MaterialPageRoute(
                                        builder: (_) => NativeResourceEditor(
                                          kind: item['kind'],
                                          item: item,
                                          productID: widget.productID,
                                        ),
                                      ),
                                    );
                                    refresh();
                                  },
                                  child: Text(m.t('edit')),
                                ),
                                if (item['status'] == 'draft')
                                  FilledButton(
                                    onPressed:
                                        item['file'] != null &&
                                            item['file']['state'] != 'ready'
                                        ? null
                                        : () async {
                                            try {
                                              await m.api(
                                                '/merchant/resources/${item['id']}/submit',
                                                method: 'POST',
                                                data: {
                                                  'revision_id':
                                                      item['revision_id'],
                                                },
                                              );
                                              refresh();
                                            } catch (e) {
                                              if (context.mounted) {
                                                showError(context, e);
                                              }
                                            }
                                          },
                                    child: Text(m.t('submit')),
                                  ),
                                if (item['visible'] == true)
                                  TextButton(
                                    onPressed: () async {
                                      final ok = await showDialog<bool>(
                                        context: context,
                                        builder: (c) => AlertDialog(
                                          content: Text(m.t('offlineConfirm')),
                                          actions: [
                                            TextButton(
                                              onPressed: () =>
                                                  Navigator.pop(c, false),
                                              child: Text(m.t('cancel')),
                                            ),
                                            TextButton(
                                              onPressed: () =>
                                                  Navigator.pop(c, true),
                                              child: Text(m.t('offline')),
                                            ),
                                          ],
                                        ),
                                      );
                                      if (ok != true) return;
                                      try {
                                        await m.api(
                                          '/merchant/resources/${item['id']}/offline',
                                          method: 'POST',
                                          data: {},
                                        );
                                        refresh();
                                      } catch (e) {
                                        if (context.mounted) {
                                          showError(context, e);
                                        }
                                      }
                                    },
                                    child: Text(m.t('offline')),
                                  ),
                                TextButton(
                                  onPressed: () => push(
                                    context,
                                    ResourceHistoryScreen(id: item['id']),
                                  ),
                                  child: Text(m.t('history')),
                                ),
                              ],
                            ],
                          ),
                        ],
                      ),
                    ),
                  ),
              Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  TextButton(
                    onPressed: page > 1 ? () => setState(() => page--) : null,
                    child: Text(m.t('previous')),
                  ),
                  Text('$page'),
                  TextButton(
                    onPressed: page * 12 < (r['total'] as num)
                        ? () => setState(() => page++)
                        : null,
                    child: Text(m.t('next')),
                  ),
                ],
              ),
            ],
          ),
        ),
      ],
    );
  }
}

class ResourceHistoryScreen extends StatelessWidget {
  final int id;
  const ResourceHistoryScreen({super.key, required this.id});
  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    return Scaffold(
      appBar: appBar(context, m.t('history')),
      body: AsyncPane(
        identity: '${m.scope}:resource-history:$id',
        load: () => m.api('/merchant/resources/$id/history'),
        builder: (r) => ListView(
          children: [
            for (final log in rows(r))
              ListTile(
                title: Text(m.t(log['action'])),
                subtitle: Text('${log['created_at']}\n${log['note']}'),
              ),
          ],
        ),
      ),
    );
  }
}

class NativeResourceViewer extends StatefulWidget {
  final Json item;
  final bool private;
  const NativeResourceViewer({
    super.key,
    required this.item,
    this.private = false,
  });
  @override
  State<NativeResourceViewer> createState() => _NativeResourceViewerState();
}

class _NativeResourceViewerState extends State<NativeResourceViewer>
    with WidgetsBindingObserver {
  VideoPlayerController? video;
  String error = '';
  bool reportedView = false;
  int page = 0;
  final pdfController = PdfViewerController();
  int pdfPage = 1, pdfCount = 0, readerVersion = 0;
  bool loading = false;
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
  }

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    if (!reportedView && !widget.private) {
      reportedView = true;
      final id = (widget.item['id'] as num?)?.toInt() ?? 0;
      if (id > 0) {
        ModelScope.of(
          context,
        ).analytics.page('/resources/$id', kind: 'resource', id: id);
      }
    }
  }

  bool videoStarting = false;
  Future<void> startVideo() async {
    if (videoStarting) return;
    setState(() {
      videoStarting = true;
      error = '';
    });
    final m = ModelScope.of(context);
    final next = VideoPlayerController.networkUrl(
      Uri.parse(
        '${m.baseUrl}/api/v1/media/${widget.item['data']['file_media_id']}',
      ),
      httpHeaders: widget.private ? {'Authorization': 'Bearer ${m.token}'} : {},
    );
    await video?.dispose();
    if (!mounted) {
      await next.dispose();
      return;
    }
    video = next;
    try {
      await next.initialize();
      if (!mounted) return;
      if (WidgetsBinding.instance.lifecycleState == null ||
          WidgetsBinding.instance.lifecycleState == AppLifecycleState.resumed) {
        await next.play();
      }
      if (mounted) setState(() {});
    } catch (_) {
      if (mounted) setState(() => error = 'network_error');
    } finally {
      if (mounted) setState(() => videoStarting = false);
    }
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state != AppLifecycleState.resumed) {
      video?.pause();
    }
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    video?.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    final d = asMap(widget.item['data']);
    return Scaffold(
      appBar: appBar(context, resourceTitle(m, widget.item)),
      body: Column(
        children: [
          if (error.isNotEmpty) message(context, m.t(error)),
          Expanded(
            child:
                widget.item['file']?['state'] != null &&
                    widget.item['file']['state'] != 'ready'
                ? Center(child: Text(m.t('noPreview')))
                : widget.item['kind'] == 'video'
                ? Center(
                    child: video?.value.isInitialized == true
                        ? Column(
                            mainAxisSize: MainAxisSize.min,
                            children: [
                              Flexible(
                                child: AspectRatio(
                                  aspectRatio: video!.value.aspectRatio,
                                  child: VideoPlayer(video!),
                                ),
                              ),
                              VideoProgressIndicator(
                                video!,
                                allowScrubbing: true,
                                padding: const EdgeInsets.all(12),
                              ),
                              FilledButton.icon(
                                onPressed: () {
                                  setState(() {
                                    video!.value.isPlaying
                                        ? video!.pause()
                                        : video!.play();
                                  });
                                },
                                icon: Icon(
                                  video!.value.isPlaying
                                      ? Icons.pause
                                      : Icons.play_arrow,
                                ),
                                label: Text(
                                  m.t(
                                    video!.value.isPlaying ? 'pause' : 'play',
                                  ),
                                ),
                              ),
                            ],
                          )
                        : videoStarting
                        ? const CircularProgressIndicator()
                        : Column(
                            mainAxisSize: MainAxisSize.min,
                            children: [
                              if ((d['cover_media_id'] ?? 0) > 0)
                                Photo(
                                  id: d['cover_media_id'],
                                  aspectRatio: 16 / 9,
                                ),
                              FilledButton.icon(
                                key: const ValueKey('video-start'),
                                onPressed: startVideo,
                                icon: const Icon(Icons.play_arrow),
                                label: Text(
                                  m.t(error.isEmpty ? 'play' : 'retry'),
                                ),
                              ),
                            ],
                          ),
                  )
                : d['format'] == 'pdf'
                ? PdfViewer.uri(
                    key: ValueKey(readerVersion),
                    params: PdfViewerParams(
                      onViewerReady: (doc, controller) {
                        WidgetsBinding.instance.addPostFrameCallback((_) {
                          if (mounted) {
                            setState(() => pdfCount = doc.pages.length);
                          }
                        });
                      },
                      onPageChanged: (value) {
                        if (value != null && value != pdfPage) {
                          WidgetsBinding.instance.addPostFrameCallback((_) {
                            if (mounted) setState(() => pdfPage = value);
                          });
                        }
                      },
                    ),
                    Uri.parse(
                      '${m.baseUrl}/api/v1/media/${d['file_media_id']}',
                    ),
                    controller: pdfController,
                    headers: widget.private
                        ? {'Authorization': 'Bearer ${m.token}'}
                        : {},
                    preferRangeAccess: true,
                  )
                : InteractiveViewer(
                    key: ValueKey('$page:$readerVersion'),
                    child: Image.network(
                      '${m.baseUrl}/api/v1/media/${d['page_media_ids'][page]}',
                      headers: widget.private
                          ? {'Authorization': 'Bearer ${m.token}'}
                          : {},
                      fit: BoxFit.contain,
                      errorBuilder: (_, _, _) => Column(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          Text(m.t('network_error')),
                          TextButton(
                            onPressed: () => setState(() => readerVersion++),
                            child: Text(m.t('retry')),
                          ),
                        ],
                      ),
                    ),
                  ),
          ),
          if (widget.item['kind'] == 'catalog')
            Wrap(
              spacing: 8,
              alignment: WrapAlignment.center,
              children: [
                if (d['format'] == 'images') ...[
                  TextButton(
                    onPressed: page > 0 ? () => setState(() => page--) : null,
                    child: Text(m.t('previous')),
                  ),
                  Text('${page + 1} / ${(d['page_media_ids'] as List).length}'),
                  TextButton(
                    onPressed: page + 1 < (d['page_media_ids'] as List).length
                        ? () => setState(() => page++)
                        : null,
                    child: Text(m.t('next')),
                  ),
                ] else ...[
                  TextButton(
                    onPressed: pdfCount > 0 && pdfPage > 1
                        ? () => pdfController.goToPage(pageNumber: pdfPage - 1)
                        : null,
                    child: Text(m.t('previous')),
                  ),
                  if (pdfCount > 0) Text('$pdfPage / $pdfCount'),
                  TextButton(
                    onPressed: pdfCount > 0 && pdfPage < pdfCount
                        ? () => pdfController.goToPage(pageNumber: pdfPage + 1)
                        : null,
                    child: Text(m.t('next')),
                  ),
                  TextButton(
                    onPressed: () => setState(() {
                      readerVersion++;
                      pdfCount = 0;
                      pdfPage = 1;
                    }),
                    child: Text(m.t('retry')),
                  ),
                  TextButton(
                    onPressed: pdfCount > 0
                        ? () => pdfController.zoomDown()
                        : null,
                    child: Text(m.t('zoomOut')),
                  ),
                  TextButton(
                    onPressed: pdfCount > 0
                        ? () => pdfController.zoomUp()
                        : null,
                    child: Text(m.t('zoomIn')),
                  ),
                  if ((d['file_media_id'] ?? 0) > 0 &&
                      (widget.private || widget.item['public_visible'] == true))
                    TextButton(
                      onPressed: loading
                          ? null
                          : () async {
                              setState(() => loading = true);
                              try {
                                final response = await m.client.get(
                                  Uri.parse(
                                    '${m.baseUrl}/api/v1/media/${d['file_media_id']}',
                                  ),
                                  headers: widget.private
                                      ? {'Authorization': 'Bearer ${m.token}'}
                                      : {},
                                );
                                if (response.statusCode != 200) {
                                  throw ApiFailure('network_error');
                                }
                                await sharingBridge.invokeMethod('saveFile', {
                                  'bytes': response.bodyBytes,
                                  'name': '${d['title']}.pdf',
                                  'mime': 'application/pdf',
                                });
                              } catch (e) {
                                if (context.mounted) showError(context, e);
                              } finally {
                                if (mounted) setState(() => loading = false);
                              }
                            },
                      child: Text(m.t('downloadPDF')),
                    ),
                ],
              ],
            ),
          if ((d['description'] ?? '').toString().isNotEmpty)
            Padding(
              padding: const EdgeInsets.all(12),
              child: Text(
                m.language == 'en' &&
                        (d['description_en'] ?? '').toString().isNotEmpty
                    ? d['description_en']
                    : d['description'],
              ),
            ),
          if ((d['product_id'] ?? 0) > 0 && !widget.private)
            TextButton(
              onPressed: () => push(
                context,
                DetailScreen(id: d['product_id'], kind: 'product'),
              ),
              child: Text(m.t('viewProduct')),
            ),
          space(12),
        ],
      ),
    );
  }
}

class ProgressMultipart extends http.MultipartRequest {
  final void Function(int, int) progress;
  ProgressMultipart(super.method, super.url, this.progress);
  @override
  http.ByteStream finalize() {
    final stream = super.finalize();
    final total = contentLength;
    var sent = 0;
    return http.ByteStream(
      stream.map((chunk) {
        sent += chunk.length;
        progress(sent, total);
        return chunk;
      }),
    );
  }
}

class NativeResourceEditor extends StatefulWidget {
  final String kind;
  final Json? item;
  final int? productID;
  const NativeResourceEditor({
    super.key,
    required this.kind,
    this.item,
    this.productID,
  });
  @override
  State<NativeResourceEditor> createState() => _NativeResourceEditorState();
}

class _NativeResourceEditorState extends State<NativeResourceEditor> {
  Json data = {}, metadata = {};
  Json? current;
  List<Json> uploads = [];
  final fields = <String, TextEditingController>{};
  bool initialized = false, busy = false, uploading = false, cancelled = false;
  int progress = 0;
  String? draftKey;
  Timer? timer;
  http.Client? uploadClient;
  int ownerID = 0;
  String sourceEndpoint = '';
  bool retired = false;
  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    final m = ModelScope.of(context);
    if (initialized) {
      if (!retired &&
          (m.baseUrl != sourceEndpoint ||
              (ownerID != 0 &&
                  m.token.isNotEmpty &&
                  m.account['id'] != ownerID))) {
        retired = true;
        cancelled = true;
        timer?.cancel();
        uploadClient?.close();
        WidgetsBinding.instance.addPostFrameCallback((_) {
          if (mounted) {
            returnToMerchant(context);
          }
        });
      }
      return;
    }
    initialized = true;
    ownerID = m.account['id'] ?? 0;
    sourceEndpoint = m.baseUrl;
    current = widget.item;
    draftKey =
        'resource.draft.${m.scope}.${widget.item?['id'] ?? '${widget.kind}.${widget.productID ?? 0}'}';
    data = widget.item == null
        ? {
            'title': '',
            'title_en': '',
            'description': '',
            'description_en': '',
            'purpose': widget.productID != null && widget.kind == 'video'
                ? 'product'
                : 'company',
            'format': widget.kind == 'video' ? 'video' : 'pdf',
            'product_id': widget.productID ?? 0,
            'file_media_id': 0,
            'cover_media_id': 0,
            'page_media_ids': <int>[],
          }
        : asMap(widget.item!['data']);
    metadata = asMap(widget.item?['file'] ?? {});
    try {
      final saved = asMap(jsonDecode(m.prefs.getString(draftKey!) ?? '{}'));
      if (saved['revision_id'] == (current?['revision_id'] ?? 0)) {
        data = asMap(saved['data']);
        uploads = rows(saved['uploads']);
      }
    } catch (_) {}
    for (final k in ['title', 'title_en', 'description', 'description_en']) {
      fields[k] = TextEditingController(text: data[k]);
      fields[k]!.addListener(() {
        data[k] = fields[k]!.text;
        unawaited(persist());
      });
    }
    timer = Timer.periodic(
      const Duration(seconds: 2),
      (_) => unawaited(poll()),
    );
  }

  Future<void> persist() async {
    if (!mounted || draftKey == null || retired) return;
    await ModelScope.of(context).prefs.setString(
      draftKey!,
      jsonEncode({
        'revision_id': current?['revision_id'] ?? 0,
        'data': data,
        'uploads': uploads,
      }),
    );
  }

  Future<void> poll() async {
    if (!mounted || retired || data['file_media_id'] == 0) return;
    final m = ModelScope.of(context);
    try {
      final v = asMap(
        await m.api('/merchant/resource-media/${data['file_media_id']}'),
      );
      if (mounted) setState(() => metadata = v);
    } catch (_) {}
  }

  @override
  void dispose() {
    timer?.cancel();
    uploadClient?.close();
    for (final f in fields.values) {
      f.dispose();
    }
    super.dispose();
  }

  Future<void> pick(String kind) async {
    try {
      final result = await FilePicker.pickFiles(
        type: FileType.custom,
        allowedExtensions: kind == 'video'
            ? ['mp4', 'mov']
            : kind == 'pdf'
            ? ['pdf']
            : ['jpg', 'jpeg', 'png', 'webp'],
      );
      if (result.isEmpty || !mounted) return;
      final root = await getApplicationDocumentsDirectory();
      final dir = Directory('${root.path}/resource-uploads');
      await dir.create(recursive: true);
      final remaining =
          30 -
          (data['page_media_ids'] as List).length -
          uploads
              .where((f) => f['kind'] == 'pages' && f['media_id'] == null)
              .length;
      if (kind == 'pages' && remaining <= 0) {
        throw ApiFailure('resource_limit_reached');
      }
      for (final f
          in (kind == 'pages' ? result.take(remaining) : result.take(1))) {
        final maxSize =
            (kind == 'video'
                ? 200
                : kind == 'pdf'
                ? 50
                : 10) *
            1024 *
            1024;
        if ((await f.length() ?? 0) > maxSize) {
          throw ApiFailure('file_too_large');
        }
        final ext = f.extension ?? 'bin';
        final path =
            '${dir.path}/${DateTime.now().microsecondsSinceEpoch}.$ext';
        var actual = 0;
        final sink = File(path).openWrite();
        try {
          await sink.addStream(
            f.readAsByteStream().map((chunk) {
              actual += chunk.length;
              if (actual > maxSize) throw ApiFailure('file_too_large');
              return chunk;
            }),
          );
        } finally {
          await sink.close();
        }
        if (actual == 0) throw ApiFailure('invalid_request');
        uploads.add({'path': path, 'name': f.name, 'kind': kind});
      }
      if (mounted) setState(() {});
      await persist();
    } catch (e) {
      if (mounted) showError(context, e);
    }
  }

  Future<void> upload() async {
    await persist();
    if (!mounted ||
        retired ||
        !await nativeSessionReady(context, ownerID, sourceEndpoint) ||
        !mounted ||
        retired) {
      return;
    }
    final m = ModelScope.of(context);
    setState(() {
      uploading = true;
      cancelled = false;
    });
    try {
      for (final file in uploads.where((f) => f['media_id'] == null)) {
        if (cancelled) break;
        uploadClient = http.Client();
        final path = file['kind'] == 'pages' || file['kind'] == 'cover'
            ? '/merchant/media'
            : '/merchant/resource-media?kind=${file['kind']}';
        final request = ProgressMultipart(
          'POST',
          Uri.parse('${m.baseUrl}/api/v1$path'),
          (sent, total) {
            final p = (sent / total * 100).floor();
            if (mounted && p != progress) setState(() => progress = p);
          },
        );
        request.headers['Authorization'] = 'Bearer ${m.token}';
        if (path == '/merchant/media') {
          request.fields['purpose'] = 'resource_cover';
        }
        request.files.add(
          await http.MultipartFile.fromPath(
            'file',
            file['path'],
            filename: file['name'],
          ),
        );
        final response = await http.Response.fromStream(
          await uploadClient!
              .send(request)
              .timeout(const Duration(minutes: 15)),
        ).timeout(const Duration(minutes: 15));
        if (response.statusCode >= 400) {
          if (response.statusCode == 401) await m.expireSession();
          final error = asMap(jsonDecode(response.body));
          throw ApiFailure(error['error']?['code'] ?? 'network_error');
        }
        final result = asMap(jsonDecode(response.body));
        final id = result['media_id'] ?? result['id'];
        file['media_id'] = id;
        if (file['kind'] == 'pages') {
          data['page_media_ids'] = [...(data['page_media_ids'] as List), id];
        } else if (file['kind'] == 'cover') {
          data['cover_media_id'] = id;
        } else {
          data['file_media_id'] = id;
          metadata = result;
        }
        await persist();
        uploadClient!.close();
        uploadClient = null;
      }
    } catch (e) {
      if (mounted && !cancelled) showError(context, e);
    } finally {
      uploadClient?.close();
      uploadClient = null;
      if (mounted) setState(() => uploading = false);
    }
  }

  Future<void> save(bool submit) async {
    await persist();
    if (!mounted ||
        retired ||
        !await nativeSessionReady(context, ownerID, sourceEndpoint) ||
        !mounted ||
        retired) {
      return;
    }
    final m = ModelScope.of(context);
    setState(() => busy = true);
    try {
      final r = asMap(
        await m.api(
          current == null
              ? '/merchant/resources'
              : '/merchant/resources/${current!['id']}/draft',
          method: current == null ? 'POST' : 'PUT',
          data: current == null
              ? {'kind': widget.kind, 'data': data}
              : {'revision_id': current!['revision_id'], 'data': data},
        ),
      );
      current = r;
      data = asMap(r['data']);
      await persist();
      if (submit) {
        await m.api(
          '/merchant/resources/${r['id']}/submit',
          method: 'POST',
          data: {'revision_id': r['revision_id']},
        );
        await m.prefs.remove(draftKey!);
        if (mounted) Navigator.pop(context, true);
      } else if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text(m.t('resourceSaved'))));
      }
    } catch (e) {
      if (mounted) showError(context, e);
    } finally {
      if (mounted) setState(() => busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    final disabled =
        retired || busy || uploading || current?['status'] == 'pending';
    return PopScope(
      onPopInvokedWithResult: (p, _) {
        if (p) unawaited(persist());
      },
      child: Scaffold(
        appBar: appBar(
          context,
          m.t(widget.kind == 'video' ? 'shortVideos' : 'electronicCatalogs'),
        ),
        body: ListView(
          padding: const EdgeInsets.all(20),
          children: [
            message(context, m.t('resourceLimit')),
            for (final k in ['title', 'description'])
              Padding(
                padding: const EdgeInsets.only(bottom: 18),
                child: NativeFieldLabel(
                  label: m.t(
                    k == 'title' ? 'resourceTitle' : 'resourceDescription',
                  ),
                  child: TextField(
                    controller: fields[k],
                    enabled: !disabled,
                    maxLength: k == 'title' ? 200 : 10000,
                    maxLines: k == 'title' ? 1 : 3,
                    decoration: InputDecoration(),
                  ),
                ),
              ),
            ExpansionTile(
              title: Text(m.t('englishInfo')),
              children: [
                NativeFieldLabel(
                  label: m.t('nameEN'),
                  child: TextField(
                    controller: fields['title_en'],
                    enabled: !disabled,
                    maxLength: 200,
                    decoration: InputDecoration(),
                  ),
                ),
                NativeFieldLabel(
                  label: m.t('englishDescription'),
                  child: TextField(
                    controller: fields['description_en'],
                    enabled: !disabled,
                    maxLength: 10000,
                    maxLines: 3,
                    decoration: InputDecoration(),
                  ),
                ),
              ],
            ),
            if (widget.kind == 'video')
              NativeFieldLabel(
                label: m.t('purpose'),
                child: DropdownButtonFormField<String>(
                  initialValue: data['purpose'],
                  isExpanded: true,
                  decoration: InputDecoration(),
                  items: [
                    for (final p in [
                      ['company', 'companyVideo'],
                      ['production', 'productionVideo'],
                      ['product', 'productVideo'],
                    ])
                      DropdownMenuItem(value: p[0], child: Text(m.t(p[1]))),
                  ],
                  onChanged: disabled
                      ? null
                      : (v) {
                          setState(() => data['purpose'] = v);
                          persist();
                        },
                ),
              )
            else
              NativeFieldLabel(
                label: m.t('catalogFormat'),
                child: DropdownButtonFormField<String>(
                  initialValue: data['format'],
                  isExpanded: true,
                  decoration: InputDecoration(),
                  items: [
                    DropdownMenuItem(value: 'pdf', child: Text(m.t('pdf'))),
                    DropdownMenuItem(
                      value: 'images',
                      child: Text(m.t('images')),
                    ),
                  ],
                  onChanged: disabled
                      ? null
                      : (v) {
                          setState(() {
                            data['format'] = v;
                            data['file_media_id'] = 0;
                            data['page_media_ids'] = <int>[];
                            metadata = {};
                            uploads = [];
                          });
                          persist();
                        },
                ),
              ),
            space(),
            Text(
              '${m.t('associatedProduct')}: ${data['product_id'] == 0 ? m.t('noAssociation') : '#${data['product_id']}'}',
            ),
            ExpansionTile(
              title: Text(m.t('searchProducts')),
              children: [
                NativeProductPicker(
                  selected: data['product_id'],
                  enabled: !disabled,
                  onSelected: (id) {
                    setState(() => data['product_id'] = id);
                    persist();
                  },
                ),
              ],
            ),
            space(),
            OutlinedButton(
              onPressed: disabled
                  ? null
                  : () => pick(
                      widget.kind == 'video'
                          ? 'video'
                          : data['format'] == 'pdf'
                          ? 'pdf'
                          : 'pages',
                    ),
              child: Text(
                m.t(
                  widget.kind == 'video'
                      ? 'uploadVideo'
                      : data['format'] == 'pdf'
                      ? 'uploadPDF'
                      : 'uploadPages',
                ),
              ),
            ),
            OutlinedButton(
              onPressed: disabled ? null : () => pick('cover'),
              child: Text(m.t('resourceCover')),
            ),
            for (final f in uploads)
              ListTile(
                title: Text(f['name']),
                subtitle: Text(
                  f['media_id'] != null ? m.t('ready') : m.t('queued'),
                ),
                trailing: f['media_id'] == null
                    ? IconButton(
                        onPressed: disabled
                            ? null
                            : () {
                                setState(() => uploads.remove(f));
                                persist();
                              },
                        icon: const Icon(Icons.close),
                      )
                    : null,
              ),
            if (data['format'] == 'images')
              for (var i = 0; i < (data['page_media_ids'] as List).length; i++)
                Row(
                  children: [
                    SizedBox(
                      width: 75,
                      child: Photo(id: data['page_media_ids'][i], height: 85),
                    ),
                    Text('${i + 1}'),
                    IconButton(
                      onPressed: disabled || i == 0
                          ? null
                          : () {
                              setState(() {
                                final a = data['page_media_ids'] as List;
                                final oldCover = a.first;
                                final v = a[i];
                                a[i] = a[i - 1];
                                a[i - 1] = v;
                                if (data['cover_media_id'] == oldCover) {
                                  data['cover_media_id'] = a.first;
                                }
                              });
                              persist();
                            },
                      icon: const Icon(Icons.arrow_upward),
                    ),
                    IconButton(
                      onPressed:
                          disabled ||
                              i + 1 == (data['page_media_ids'] as List).length
                          ? null
                          : () {
                              setState(() {
                                final a = data['page_media_ids'] as List;
                                final oldCover = a.first;
                                final v = a[i];
                                a[i] = a[i + 1];
                                a[i + 1] = v;
                                if (data['cover_media_id'] == oldCover) {
                                  data['cover_media_id'] = a.first;
                                }
                              });
                              persist();
                            },
                      icon: const Icon(Icons.arrow_downward),
                    ),
                    IconButton(
                      onPressed: disabled
                          ? null
                          : () {
                              setState(() {
                                final id = (data['page_media_ids'] as List)
                                    .removeAt(i);
                                if (data['cover_media_id'] == id) {
                                  data['cover_media_id'] = 0;
                                }
                                uploads.removeWhere((f) => f['media_id'] == id);
                              });
                              persist();
                            },
                      icon: const Icon(Icons.close),
                    ),
                  ],
                ),
            if (uploads.any((f) => f['media_id'] == null) && !uploading)
              FilledButton(
                onPressed: disabled ? null : upload,
                child: Text(m.t('uploadFile')),
              ),
            if (uploading) ...[
              LinearProgressIndicator(value: progress / 100),
              Text('${m.t('uploadProgress')} $progress%'),
              TextButton(
                onPressed: () {
                  cancelled = true;
                  uploadClient?.close();
                },
                child: Text(m.t('cancel')),
              ),
            ],
            if (metadata.isNotEmpty)
              message(
                context,
                '${m.t(metadata['state'])}\n${m.t(metadata['error'] ?? '')}',
              ),
            if (metadata['state'] == 'failed')
              TextButton(
                onPressed: disabled
                    ? null
                    : () async {
                        try {
                          await m.api(
                            '/merchant/resource-media/${metadata['media_id']}/retry',
                            method: 'POST',
                            data: {},
                          );
                          poll();
                        } catch (e) {
                          if (context.mounted) showError(context, e);
                        }
                      },
                child: Text(m.t('retryProcessing')),
              ),
            message(context, m.t('resourceReview')),
            OutlinedButton(
              onPressed:
                  (data['file_media_id'] != 0 ||
                      (data['page_media_ids'] as List).isNotEmpty)
                  ? () => push(
                      context,
                      NativeResourceViewer(
                        item: {
                          'kind': widget.kind,
                          'data': data,
                          'file': metadata,
                        },
                        private: true,
                      ),
                    )
                  : null,
              child: Text(m.t('resourcePreview')),
            ),
            space(),
            OutlinedButton(
              onPressed: disabled ? null : () => save(false),
              child: Text(m.t('saveCloud')),
            ),
            space(),
            FilledButton(
              onPressed:
                  disabled ||
                      metadata.isNotEmpty && metadata['state'] != 'ready'
                  ? null
                  : () => save(true),
              child: Text(m.t('submit')),
            ),
            space(24),
          ],
        ),
      ),
    );
  }
}

class NativeProductPicker extends StatefulWidget {
  final int selected;
  final bool enabled;
  final ValueChanged<int> onSelected;
  const NativeProductPicker({
    super.key,
    required this.selected,
    required this.enabled,
    required this.onSelected,
  });
  @override
  State<NativeProductPicker> createState() => _NativeProductPickerState();
}

class _NativeProductPickerState extends State<NativeProductPicker> {
  int page = 1;
  String q = '';
  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    return Column(
      children: [
        TextButton(
          onPressed: widget.enabled ? () => widget.onSelected(0) : null,
          child: Text(m.t('noAssociation')),
        ),
        NativeFieldLabel(
          label: m.t('keyword'),
          child: TextField(
            decoration: InputDecoration(),
            onChanged: (v) => setState(() {
              q = v;
              page = 1;
            }),
          ),
        ),
        AsyncPane(
          identity: '${m.scope}:pick:$q:$page',
          load: () => m.api(
            '/merchant/contents?kind=product&page_size=20&page=$page&q=${Uri.encodeComponent(q)}',
          ),
          builder: (r) => Column(
            children: [
              for (final i in rows(r['items']))
                ListTile(
                  title: Text(m.content(asMap(i['data']), 'name')),
                  selected: i['id'] == widget.selected,
                  onTap: widget.enabled
                      ? () => widget.onSelected(i['id'])
                      : null,
                ),
              Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  TextButton(
                    onPressed: page > 1 ? () => setState(() => page--) : null,
                    child: Text(m.t('previous')),
                  ),
                  TextButton(
                    onPressed: page * 20 < (r['total'] as num)
                        ? () => setState(() => page++)
                        : null,
                    child: Text(m.t('next')),
                  ),
                ],
              ),
            ],
          ),
        ),
      ],
    );
  }
}
