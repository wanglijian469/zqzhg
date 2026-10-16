part of 'main.dart';

class BrowseScreen extends StatefulWidget {
  final String query;
  final int? category;
  final String kind;
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
  int page = 1;
  int total = 0, pageSize = 20;
  final scroll = ScrollController();
  Timer? debounce;
  String? endpoint, requestKey;
  late Future<dynamic> result;
  late Future<dynamic> taxonomy;

  @override
  void initState() {
    super.initState();
    query = TextEditingController(text: widget.query);
    term = widget.query.trim();
    category = widget.category;
  }

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    final m = ModelScope.of(context);
    if (m.isHallPreview) {
      debounce?.cancel();
      return;
    }
    if (endpoint != m.baseUrl) {
      endpoint = m.baseUrl;
      taxonomy = m.publicTaxonomy();
    }
    load();
  }

  @override
  void didUpdateWidget(BrowseScreen oldWidget) {
    super.didUpdateWidget(oldWidget);
    final m = ModelScope.of(context);
    if (!m.isHallPreview) taxonomy = m.publicTaxonomy();
  }

  void load({bool force = false}) {
    final m = ModelScope.of(context);
    if (m.isHallPreview) return;
    final params = Uri(
      queryParameters: {
        'q': term,
        'page': '$page',
        'page_size': '20',
        if (category != null) 'category_id': '$category',
        if (region != null) 'region_id': '$region',
      },
    ).query;
    final key = '${m.baseUrl}:${widget.kind}:$params';
    if (!force && requestKey == key) return;
    requestKey = key;
    result = m.api(
      '/public/${widget.kind == 'product' ? 'products' : 'vendors'}?$params',
    );
  }

  void update(VoidCallback change) {
    debounce?.cancel();
    setState(() {
      change();
      load();
    });
    if (scroll.hasClients) scroll.jumpTo(0);
  }

  void searchFor(String value, {int? categoryID}) {
    update(() {
      query.text = value;
      term = value.trim();
      category = categoryID;
      region = null;
      page = 1;
    });
  }

  void submit(String value) => update(() {
    term = value.trim();
    page = 1;
  });

  @override
  void dispose() {
    debounce?.cancel();
    query.dispose();
    scroll.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
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
              taxonomy = m.publicTaxonomy(refresh: true);
              load(force: true);
            });
            await Future.wait(
              [result, taxonomy].map(
                (f) =>
                    f.then<void>((_) {}, onError: (Object e, StackTrace s) {}),
              ),
            );
          },
          child: CustomScrollView(
            controller: scroll,
            key: PageStorageKey('browse-${widget.kind}'),
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
                        labelText: m.t('keyword'),
                        suffixIcon: IconButton(
                          tooltip: m.t('homeSearchAction'),
                          onPressed: () => submit(query.text),
                          icon: const Icon(Icons.search),
                        ),
                      ),
                      onChanged: (value) {
                        debounce?.cancel();
                        debounce = Timer(const Duration(milliseconds: 350), () {
                          if (mounted) submit(value);
                        });
                      },
                      onSubmitted: submit,
                    ),
                    space(12),
                    FutureBuilder<dynamic>(
                      future: taxonomy,
                      builder: (context, snap) {
                        if (snap.hasError) {
                          return HomeLoadState(
                            error: snap.error,
                            onRetry: () => setState(() {
                              taxonomy = m.publicTaxonomy(refresh: true);
                            }),
                          );
                        }
                        if (!snap.hasData) return const HomeLoadState();
                        final filters = [
                          DropdownButtonFormField<int>(
                            key: ValueKey('category:$category'),
                            initialValue: category,
                            isExpanded: true,
                            decoration: InputDecoration(
                              labelText: m.t('category'),
                              contentPadding: const EdgeInsets.symmetric(
                                horizontal: 10,
                                vertical: 10,
                              ),
                            ),
                            items: [
                              DropdownMenuItem<int>(
                                value: null,
                                child: Text(m.t('allCategories')),
                              ),
                              ...rows(snap.data['categories']).map(
                                (c) => DropdownMenuItem<int>(
                                  value: c['id'],
                                  child: Text(
                                    m.content(c, 'name'),
                                    overflow: TextOverflow.ellipsis,
                                  ),
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
                            initialValue: region,
                            isExpanded: true,
                            decoration: InputDecoration(
                              labelText: m.t('region'),
                              contentPadding: const EdgeInsets.symmetric(
                                horizontal: 10,
                                vertical: 10,
                              ),
                            ),
                            items: [
                              DropdownMenuItem<int>(
                                value: null,
                                child: Text(m.t('allRegions')),
                              ),
                              ...rows(snap.data['regions']).map(
                                (r) => DropdownMenuItem<int>(
                                  value: r['id'],
                                  child: Text(
                                    m.content(r, 'name'),
                                    overflow: TextOverflow.ellipsis,
                                  ),
                                ),
                              ),
                            ],
                            onChanged: (v) => update(() {
                              region = v;
                              page = 1;
                            }),
                          ),
                        ];
                        if (MediaQuery.textScalerOf(context).scale(14) / 14 >=
                            1.3) {
                          return Column(
                            children: [filters[0], space(12), filters[1]],
                          );
                        }
                        return Row(
                          children: [
                            Expanded(child: filters[0]),
                            const SizedBox(width: 10),
                            Expanded(child: filters[1]),
                          ],
                        );
                      },
                    ),
                    space(12),
                  ],
                ),
              ),
              FutureBuilder<dynamic>(
                future: result,
                builder: (context, snap) {
                  if (!snap.hasData || snap.hasError) {
                    return SliverToBoxAdapter(
                      child: Padding(
                        padding: const EdgeInsets.all(16),
                        child: HomeLoadState(
                          error: snap.error,
                          onRetry: () => setState(() => load(force: true)),
                        ),
                      ),
                    );
                  }
                  final items = rows(snap.data['items']);
                  total = (snap.data['total'] as num? ?? 0).toInt();
                  pageSize = (snap.data['page_size'] as num? ?? 20).toInt();
                  return SliverPadding(
                    padding: const EdgeInsets.symmetric(horizontal: 16),
                    sliver: SliverMainAxisGroup(
                      slivers: [
                        SliverToBoxAdapter(
                          child: Text(
                            '${m.t(widget.kind == 'product' ? 'productResults' : 'results')}: $total',
                            style: Theme.of(context).textTheme.bodySmall,
                          ),
                        ),
                        if (items.isEmpty)
                          SliverToBoxAdapter(
                            child: message(
                              context,
                              m.t(
                                widget.kind == 'product'
                                    ? 'noProductResults'
                                    : 'noResults',
                              ),
                            ),
                          ),
                        if (widget.kind == 'product')
                          ExhibitSliver(items: items)
                        else
                          SliverList.builder(
                            itemCount: items.length,
                            itemBuilder: (context, index) =>
                                NativeVendorCard(item: items[index]),
                          ),
                        SliverToBoxAdapter(
                          child: Padding(
                            padding: const EdgeInsets.symmetric(vertical: 16),
                            child: Row(
                              mainAxisAlignment: MainAxisAlignment.spaceBetween,
                              children: [
                                TextButton(
                                  onPressed: page > 1
                                      ? () => update(() => page--)
                                      : null,
                                  child: Text(m.t('previous')),
                                ),
                                Text('$page'),
                                TextButton(
                                  onPressed: page * pageSize < total
                                      ? () => update(() => page++)
                                      : null,
                                  child: Text(m.t('next')),
                                ),
                              ],
                            ),
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
