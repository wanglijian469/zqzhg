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
      page = (old?['page'] ?? 1) as int;
      requestKey = null;
      taxonomy = m.publicTaxonomy();
      final offset = (old?['offset'] as num?)?.toDouble() ?? 0;
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
    return NativeFilterPanel(
      count:
          (category == null ? 0 : 1) +
          (region == null ? 0 : 1) +
          (order.isEmpty ? 0 : 1),
      panelKey: PageStorageKey('filters:$endpoint'),
      children: [
        DropdownButtonFormField<int>(
          key: ValueKey('category:$category'),
          initialValue: cats.any((c) => c['id'] == category) ? category : null,
          isExpanded: true,
          decoration: InputDecoration(labelText: m.t('category')),
          items: [
            DropdownMenuItem(value: null, child: Text(m.t('allCategories'))),
            for (final c in cats)
              DropdownMenuItem(
                value: c['id'],
                child: Text(
                  m.content(c, 'name'),
                  overflow: TextOverflow.ellipsis,
                ),
              ),
          ],
          onChanged: (v) => update(() {
            category = v;
            page = 1;
          }),
        ),
        DropdownButtonFormField<int>(
          key: ValueKey('region:$region'),
          initialValue: regions.any((r) => r['id'] == region) ? region : null,
          isExpanded: true,
          decoration: InputDecoration(labelText: m.t('region')),
          items: [
            DropdownMenuItem(value: null, child: Text(m.t('allRegions'))),
            for (final r in regions)
              DropdownMenuItem(
                value: r['id'],
                child: Text(
                  m.content(r, 'name'),
                  overflow: TextOverflow.ellipsis,
                ),
              ),
          ],
          onChanged: (v) => update(() {
            region = v;
            page = 1;
          }),
        ),
        if (m.modernPublicApi)
          DropdownButtonFormField<String>(
            initialValue: order,
            isExpanded: true,
            decoration: InputDecoration(labelText: m.t('sort')),
            items: [
              DropdownMenuItem(value: '', child: Text(m.t('defaultSort'))),
              DropdownMenuItem(value: 'latest', child: Text(m.t('latestSort'))),
              if (widget.kind == 'vendor')
                DropdownMenuItem(
                  value: 'name',
                  child: Text(m.t('companyNameSort')),
                ),
            ],
            onChanged: (v) => update(() {
              order = v ?? '';
              page = 1;
            }),
          ),
        if (category != null || region != null || order.isNotEmpty)
          TextButton(
            onPressed: () => update(() {
              category = null;
              region = null;
              order = '';
              page = 1;
            }),
            child: Text(m.t('clear')),
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
        extraActions:[if(widget.kind=='vendor')IconButton(tooltip:syncText(context,'厂商地图','Manufacturer map'),onPressed:()=>push(context,const NativeIndustryMapScreen()),icon:const Icon(Icons.map_outlined))],
      ),
      body: SafeArea(
        top: false,
        bottom: false,
        child: RefreshIndicator(
          onRefresh: () async {
            setState(() {
              taxonomy = m.publicTaxonomy(refresh: true);
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
                padding: const EdgeInsets.fromLTRB(16, 12, 16, 0),
                sliver: SliverList.list(
                  children: [
                    TextField(
                      controller: query,
                      textInputAction: TextInputAction.search,
                      decoration: InputDecoration(
                        labelText: m.t(
                          widget.kind == 'product'
                              ? 'showroomKeyword'
                              : 'keyword',
                        ),
                        suffixIcon: IconButton(
                          tooltip: m.t('homeSearchAction'),
                          onPressed: () => submit(query.text),
                          icon: const Icon(Icons.search),
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
                              () => taxonomy = m.publicTaxonomy(refresh: true),
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
                    padding: const EdgeInsets.symmetric(horizontal: 16),
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
                          SliverList.builder(
                            itemCount: items.length,
                            itemBuilder: (c, i) =>
                                NativeVendorCard(item: items[i]),
                          ),
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
