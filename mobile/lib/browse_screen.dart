part of 'main.dart';

class BrowseScreen extends StatefulWidget {
  final String query, kind;
  final int? category;
  const BrowseScreen({
    super.key,
    this.query = '',
    this.category,
    this.kind = 'vendor',
  });
  @override
  State<BrowseScreen> createState() => _BrowseScreenState();
}

class _BrowseScreenState extends State<BrowseScreen> {
  late final TextEditingController query;
  late String term;
  int? category, region;
  String order = '';
  int page = 1, total = 0, pageSize = 20;
  final scroll = ScrollController();
  String? endpoint, requestKey;
  String lastSearch = '';
  Future<dynamic>? result, taxonomy;
  @override
  void initState() {
    super.initState();
    query = TextEditingController(text: widget.query);
    term = widget.query.trim();
    category = widget.category;
  }

  void save() {
    if (endpoint == null) return;
    ModelScope.of(context).browseMemory[endpoint!] = {
      'query': query.text,
      'term': term,
      'category': category,
      'region': region,
      'order': order,
      'page': page,
      'offset': scroll.hasClients ? scroll.offset : 0.0,
    };
  }

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    final m = ModelScope.of(context);
    unawaited(m.loadHalls());
    if (!m.hallsReady || m.isHallPreview) return;
    final scope = '${m.baseUrl}:${m.activeHall.slug}:${widget.kind}';
    if (endpoint != scope) {
      save();
      endpoint = scope;
      final old = m.browseMemory[scope];
      query.text = (old?['query'] ?? widget.query).toString();
      term = (old?['term'] ?? widget.query.trim()).toString();
      category = old?['category'] ?? (old == null ? widget.category : null);
      region = old?['region'];
      order = (old?['order'] ?? '').toString();
      final obsolete =
          old?['belt'] != null ||
          (old?['business_nature'] ?? '').toString().isNotEmpty;
      page = obsolete ? 1 : (old?['page'] ?? 1) as int;
      if (obsolete) {
        final cleaned = Map<String, dynamic>.from(old!);
        cleaned.remove('belt');
        cleaned.remove('business_nature');
        cleaned['page'] = 1;
        cleaned['offset'] = 0.0;
        m.browseMemory[scope] = cleaned;
      }
      requestKey = null;
      taxonomy = m.publicTaxonomy(kind: widget.kind);
      final categoryScope = scope;
      unawaited(
        taxonomy!.then<void>((value) {
          if (!mounted || endpoint != categoryScope || category == null) return;
          final data = asMap(value), aliases = rows(asMap(value)['aliases']);
          final alias = aliases
              .where((a) => a['source_id'] == category)
              .firstOrNull;
          final canonical = alias?['target_id'] ?? category;
          final valid = rows(
            data['categories'],
          ).any((c) => c['id'] == canonical);
          if (!valid || canonical != category) {
            setState(() {
              category = valid ? (canonical as num).toInt() : null;
              page = 1;
              requestKey = null;
              load();
              save();
            });
            if (!valid) {
              ScaffoldMessenger.of(context).showSnackBar(
                SnackBar(
                  content: Text(
                    syncText(
                      context,
                      '原分类已更新，请重新选择类别。',
                      'Categories have changed. Please select a category again.',
                    ),
                  ),
                ),
              );
            }
          }
        }, onError: (Object e, StackTrace s) {}),
      );
      final offset = obsolete
          ? 0.0
          : ((old?['offset'] as num?)?.toDouble() ?? 0);
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (mounted && endpoint == scope && scroll.hasClients) {
          scroll.jumpTo(min(offset, scroll.position.maxScrollExtent));
        }
      });
    }
    load();
  }

  void load({bool force = false}) {
    final m = ModelScope.of(context);
    if (!m.hallsReady || m.isHallPreview) return;
    pageSize = widget.kind == 'product' ? 24 : 20;
    final params = Uri(
      queryParameters: {
        'q': term,
        'page': '$page',
        'page_size': '$pageSize',
        if (category != null) 'category_id': '$category',
        if (region != null) 'region_id': '$region',
        if (m.modernPublicApi && order.isNotEmpty) 'sort': order,
      },
    ).query;
    final key = '$endpoint:$params';
    if (!force && requestKey == key) return;
    requestKey = key;
    result = m.publicApi(
      '/public/${widget.kind == 'product' ? 'products' : 'vendors'}?$params',
    );
    if (term.isNotEmpty && lastSearch != term) {
      lastSearch = term;
      m.analytics.event(
        'search',
        widget.kind == 'product' ? '/browse/products' : '/browse',
        keyword: term,
      );
    }
  }

  void update(VoidCallback change) {
    setState(() {
      change();
      load();
    });
    if (scroll.hasClients) scroll.jumpTo(0);
    save();
  }

  void searchFor(String value, {int? categoryID}) => update(() {
    query.text = value;
    term = value.trim();
    category = categoryID;
    region = null;
    order = '';
    page = 1;
  });
  void submit(String value) {
    FocusScope.of(context).unfocus();
    update(() {
      term = value.trim();
      page = 1;
    });
  }

  @override
  void dispose() {
    query.dispose();
    scroll.dispose();
    super.dispose();
  }

  Widget filters(AppModel m, Json tax) {
    final cats = rows(tax['categories']), regions = rows(tax['regions']);
    Widget button(String label, bool selected, VoidCallback open, {Key? key}) =>
        Expanded(
          child: TextButton(
            key: key,
            onPressed: open,
            style: TextButton.styleFrom(
              minimumSize: const Size(48, 48),
              padding: const EdgeInsets.symmetric(horizontal: 4),
              foregroundColor: selected ? industrialBlue : industrialInk,
            ),
            child: Row(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                Flexible(
                  child: Text(
                    label,
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                    textAlign: TextAlign.center,
                  ),
                ),
                const Icon(Icons.expand_more, size: 16),
              ],
            ),
          ),
        );
    Future<void> chooseInt(bool isCategory) async {
      FocusScope.of(context).unfocus();
      final value = await nativeFilterSheet<int>(
        context,
        title: isCategory
            ? widget.kind == 'vendor'
                  ? syncText(context, '厂家分类', 'Manufacturer category')
                  : m.t('category')
            : syncText(context, '所在地区', 'Region'),
        selected: (isCategory ? category : region) ?? 0,
        options: [
          (value: 0, label: m.t(isCategory ? 'allCategories' : 'allRegions')),
          for (final row in isCategory ? cats : regions)
            (
              value: (row['id'] as num).toInt(),
              label: isCategory
                  ? nativeCategoryPath(cats, row['id'], m)
                  : m.content(row, 'name'),
            ),
        ],
      );
      if (value == null || !mounted) return;
      update(() {
        if (isCategory) {
          category = value == 0 ? null : value;
        } else {
          region = value == 0 ? null : value;
        }
        page = 1;
      });
    }

    final selectedCategory = cats.where((c) => c['id'] == category).firstOrNull;
    final selectedRegion = regions.where((r) => r['id'] == region).firstOrNull;
    return Column(
      children: [
        Row(
          children: [
            button(
              selectedCategory == null
                  ? syncText(
                      context,
                      widget.kind == 'vendor' ? '厂家分类' : '产品分类',
                      'Category',
                    )
                  : m.content(selectedCategory, 'name'),
              category != null,
              () => chooseInt(true),
              key: const ValueKey('browse-category-filter'),
            ),
            button(
              selectedRegion == null
                  ? syncText(context, '地区', 'Region')
                  : m.content(selectedRegion, 'name'),
              region != null,
              () => chooseInt(false),
            ),
            if (m.modernPublicApi)
              button(
                m.t(
                  order == 'latest'
                      ? 'latestSort'
                      : order == 'name'
                      ? 'companyNameSort'
                      : 'sort',
                ),
                order.isNotEmpty,
                () async {
                  FocusScope.of(context).unfocus();
                  final value = await nativeFilterSheet<String>(
                    context,
                    title: m.t('sort'),
                    selected: order,
                    options: [
                      (value: '', label: m.t('defaultSort')),
                      (value: 'latest', label: m.t('latestSort')),
                      if (widget.kind == 'vendor')
                        (value: 'name', label: m.t('companyNameSort')),
                    ],
                  );
                  if (value != null && mounted) {
                    update(() {
                      order = value;
                      page = 1;
                    });
                  }
                },
              ),
          ],
        ),
        if (category != null || region != null || order.isNotEmpty)
          Align(
            alignment: Alignment.centerRight,
            child: TextButton(
              onPressed: () => update(() {
                category = null;
                region = null;
                order = '';
                page = 1;
              }),
              child: Text(m.t('clear')),
            ),
          ),
      ],
    );
  }

  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    if (!m.hallsReady) return const HallLoadScreen();
    if (m.isHallPreview) return const HallComingSoonScreen();
    return Scaffold(
      appBar: appBar(
        context,
        m.t(widget.kind == 'product' ? 'navProducts' : 'navManufacturers'),
      ),
      body: SafeArea(
        top: false,
        bottom: false,
        child: RefreshIndicator(
          onRefresh: () async {
            setState(() {
              taxonomy = m.publicTaxonomy(refresh: true, kind: widget.kind);
              load(force: true);
            });
            await Future.wait(
              [result!, taxonomy!].map(
                (f) =>
                    f.then<void>((_) {}, onError: (Object e, StackTrace s) {}),
              ),
            );
          },
          child: CustomScrollView(
            controller: scroll,
            key: PageStorageKey('browse:$endpoint'),
            physics: const AlwaysScrollableScrollPhysics(),
            slivers: [
              SliverPadding(
                padding: const EdgeInsets.fromLTRB(12, 8, 12, 0),
                sliver: SliverList.list(
                  children: [
                    TextField(
                      key: const ValueKey('browse-search'),
                      controller: query,
                      textInputAction: TextInputAction.search,
                      decoration: InputDecoration(
                        hintText: syncText(
                          context,
                          widget.kind == 'product'
                              ? '搜索产品名称、型号'
                              : '搜索厂家名称、主营产品',
                          widget.kind == 'product'
                              ? 'Product name or model'
                              : 'Manufacturer or products',
                        ),
                        prefixIcon: const Icon(Icons.search),
                        suffixIcon: IconButton(
                          tooltip: m.t('homeSearchAction'),
                          onPressed: () => submit(query.text),
                          icon: const Icon(Icons.arrow_forward),
                        ),
                      ),
                      onSubmitted: submit,
                    ),
                    FutureBuilder<dynamic>(
                      key: ValueKey('taxonomy:$endpoint'),
                      future: taxonomy,
                      builder: (context, snap) {
                        if (snap.hasError) {
                          return HomeLoadState(
                            error: snap.error,
                            onRetry: () => setState(
                              () => taxonomy = m.publicTaxonomy(
                                refresh: true,
                                kind: widget.kind,
                              ),
                            ),
                          );
                        }
                        if (!snap.hasData) return const HomeLoadState();
                        return filters(m, asMap(snap.data));
                      },
                    ),
                  ],
                ),
              ),
              FutureBuilder<dynamic>(
                key: ValueKey(requestKey),
                future: result,
                builder: (context, snap) {
                  if (!snap.hasData || snap.hasError) {
                    return SliverToBoxAdapter(
                      child: HomeLoadState(
                        error: snap.error,
                        onRetry: () => setState(() => load(force: true)),
                      ),
                    );
                  }
                  final items = uniqueContent(rows(snap.data['items']));
                  total = (snap.data['total'] as num? ?? 0).toInt();
                  pageSize = (snap.data['page_size'] as num? ?? pageSize)
                      .toInt();
                  return SliverPadding(
                    padding: const EdgeInsets.symmetric(horizontal: 12),
                    sliver: SliverMainAxisGroup(
                      slivers: [
                        SliverToBoxAdapter(
                          child: Padding(
                            padding: const EdgeInsets.symmetric(vertical: 8),
                            child: Text(
                              '${m.t(widget.kind == 'product' ? 'productResults' : 'results')}: $total',
                              style: Theme.of(context).textTheme.bodySmall,
                            ),
                          ),
                        ),
                        if (items.isEmpty)
                          SliverToBoxAdapter(
                            child: Column(
                              children: [
                                message(
                                  context,
                                  m.t(
                                    widget.kind == 'product'
                                        ? 'noProductResults'
                                        : 'noResults',
                                  ),
                                ),
                                if (term.isNotEmpty ||
                                    category != null ||
                                    region != null)
                                  TextButton(
                                    onPressed: () => searchFor(''),
                                    child: Text(m.t('clear')),
                                  ),
                              ],
                            ),
                          ),
                        if (widget.kind == 'product')
                          ExhibitSliver(items: items)
                        else
                          VendorSliver(items: items),
                        SliverToBoxAdapter(
                          child: NativePagination(
                            page: page,
                            total: total,
                            size: pageSize,
                            onPage: (n) => update(() => page = n),
                          ),
                        ),
                      ],
                    ),
                  );
                },
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class NativePagination extends StatelessWidget {
  final int page, total, size;
  final ValueChanged<int> onPage;
  const NativePagination({
    super.key,
    required this.page,
    required this.total,
    required this.size,
    required this.onPage,
  });
  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    if (total <= size) return const SizedBox();
    final label = Text('$page / ${max(1, (total / size).ceil())}');
    final previous = Expanded(
      child: TextButton(
        onPressed: page > 1 ? () => onPage(page - 1) : null,
        child: Text(m.t('previous'), textAlign: TextAlign.center),
      ),
    );
    final next = Expanded(
      child: TextButton(
        onPressed: page * size < total ? () => onPage(page + 1) : null,
        child: Text(m.t('next'), textAlign: TextAlign.center),
      ),
    );
    final large = MediaQuery.textScalerOf(context).scale(14) / 14 >= 1.3;
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 16),
      child: large
          ? Column(
              children: [
                label,
                Row(children: [previous, next]),
              ],
            )
          : Row(children: [previous, label, next]),
    );
  }
}
