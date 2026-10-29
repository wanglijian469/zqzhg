part of 'main.dart';

class IndustrialHomeScreen extends StatefulWidget {
  const IndustrialHomeScreen({super.key});
  @override
  State<IndustrialHomeScreen> createState() => _IndustrialHomeScreenState();
}

class _IndustrialHomeScreenState extends State<IndustrialHomeScreen> {
  final query = TextEditingController();
  String kind = 'vendor';
  String? endpoint;
  Future<dynamic>? home;
  final homes = <String, Future<dynamic>>{};

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    final m = ModelScope.of(context);
    unawaited(m.loadHalls());
    if (!m.hallsReady) return;
    final homeScope = '${m.baseUrl}:${m.activeHall.slug}';
    if (endpoint != homeScope) {
      endpoint = homeScope;
      home = homes[homeScope];
    }
    if (!m.isHallPreview && home == null) {
      home = homes[homeScope] = m.publicApi('/public/home');
      unawaited(
        m
            .publicTaxonomy(kind: 'product')
            .then<void>((_) {}, onError: (Object e, StackTrace s) {}),
      );
    }
  }

  @override
  void dispose() {
    query.dispose();
    super.dispose();
  }

  Future<void> refresh() async {
    final m = ModelScope.of(context);
    if (m.isHallPreview) return;
    setState(() {
      home = homes['${m.baseUrl}:${m.activeHall.slug}'] = m.publicApi(
        '/public/home',
      );
    });
    await Future.wait(
      [
        home!,
      ].map((f) => f.then<void>((_) {}, onError: (Object e, StackTrace s) {})),
    );
  }

  void browse(String type, {String? term, int? category}) {
    FocusScope.of(context).unfocus();
    final navigation = AppNavigation.of(context);
    if (navigation != null) {
      navigation.browse(type, query: term ?? '', category: category);
    } else {
      push(
        context,
        BrowseScreen(kind: type, query: term ?? '', category: category),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    if (!m.hallsReady) return const HallLoadScreen();
    if (m.isHallPreview) return const HallComingSoonScreen();
    return Scaffold(
      appBar: const HomeBrandBar(),
      body: SafeArea(
        top: false,
        bottom: false,
        child: RefreshIndicator(
          onRefresh: refresh,
          child: CustomScrollView(
            key: PageStorageKey('exhibition-home:${m.activeHall.slug}'),
            physics: const AlwaysScrollableScrollPhysics(),
            slivers: [
              SliverPadding(
                padding: const EdgeInsets.fromLTRB(16, 8, 16, 0),
                sliver: SliverList.list(
                  children: [
                    HomeSearchField(
                      query: query,
                      kind: kind,
                      onKind: (v) => setState(() => kind = v),
                      onSearch: () => browse(kind, term: query.text.trim()),
                    ),

                    const HomeQuickActions(),
                    space(8),
                    FutureBuilder<dynamic>(
                      key: ValueKey('hero:$endpoint'),
                      future: home,
                      builder: (context, snap) {
                        final data = asMap(snap.data ?? {});
                        final hero = asMap(data['hero'] ?? {});
                        return HomeBrandBanner(
                          mediaID: (hero['media_id'] as num?)?.toInt(),
                        );
                      },
                    ),
                  ],
                ),
              ),
              FutureBuilder<dynamic>(
                key: ValueKey('home:$endpoint'),
                future: home,
                builder: (context, snap) {
                  if (!snap.hasData || snap.hasError) {
                    return SliverToBoxAdapter(
                      child: Padding(
                        padding: const EdgeInsets.all(16),
                        child: HomeLoadState(
                          error: snap.error,
                          onRetry: () => setState(() {
                            home = m.publicApi('/public/home');
                          }),
                        ),
                      ),
                    );
                  }
                  final data = asMap(snap.data);
                  final allVendors = rows(data['vendors']);
                  final vendors = allVendors.take(12).toList();
                  final products = uniqueContent(rows(data['products']))
                      .take(8)
                      .map((p) {
                        final vendor = allVendors
                            .where((v) => v['id'] == p['vendor_id'])
                            .firstOrNull;
                        return <String, dynamic>{
                          ...p,
                          if (p['vendor'] == null && vendor != null)
                            'vendor': vendor,
                        };
                      })
                      .toList();
                  return SliverPadding(
                    padding: const EdgeInsets.symmetric(horizontal: 16),
                    sliver: SliverMainAxisGroup(
                      slivers: [
                        SliverToBoxAdapter(
                          child: IndustrialSectionTitle(
                            title: m.t('homeRecommended'),
                            onAll: () => browse('vendor'),
                          ),
                        ),
                        if (vendors.isEmpty)
                          SliverToBoxAdapter(
                            child: message(context, m.t('emptyDesc')),
                          ),
                        VendorSliver(
                          items: vendors,
                          productImages: rows(data['products']),
                        ),
                        SliverToBoxAdapter(
                          child: IndustrialSectionTitle(
                            title: m.t('popular'),
                            onAll: () => browse('product'),
                          ),
                        ),
                        if (products.isEmpty)
                          SliverToBoxAdapter(
                            child: message(context, m.t('emptyProducts')),
                          ),
                        ExhibitSliver(items: products),
                      ],
                    ),
                  );
                },
              ),
              SliverToBoxAdapter(
                child: Padding(
                  padding: const EdgeInsets.fromLTRB(16, 22, 16, 24),
                  child: Column(
                    children: [
                      Text(
                        m.t('footerNote'),
                        textAlign: TextAlign.center,
                        style: Theme.of(context).textTheme.bodySmall,
                      ),
                    ],
                  ),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class HomeBrandBar extends StatelessWidget implements PreferredSizeWidget {
  const HomeBrandBar({super.key});
  @override
  Size get preferredSize => const Size.fromHeight(64);
  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    return AppBar(
      toolbarHeight: 64,
      automaticallyImplyLeading: false,
      titleSpacing: 16,
      title: Text(
        '${m.t('brand')}-${exhibitionHallName(m, m.activeHall)}',
        maxLines: 2,
        style: const TextStyle(fontSize: 18, fontWeight: FontWeight.w700),
      ),
      actions: [
        if (m.showHallSelector)
          PopupMenuButton<ExhibitionHall>(
            key: const ValueKey('home-hall-selector'),
            tooltip: m.t('chooseHall'),
            onSelected: m.selectHall,
            itemBuilder: (_) => [
              for (final hall in m.availableHalls.where((h) => h.isOpen))
                PopupMenuItem(
                  value: hall,
                  key: ValueKey('hall-option-${hall.name}'),
                  child: Text(
                    '${exhibitionHallName(m, hall)}${hall.isOpen ? '' : ' · ${m.t('hallPreparing')}'}',
                  ),
                ),
            ],
            icon: const Icon(Icons.domain_outlined),
          ),
      ],
    );
  }
}

class HomeSearchField extends StatelessWidget {
  final TextEditingController query;
  final String kind;
  final ValueChanged<String> onKind;
  final VoidCallback onSearch;
  const HomeSearchField({
    super.key,
    required this.query,
    required this.kind,
    required this.onKind,
    required this.onSearch,
  });
  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    return TextField(
      key: const ValueKey('home-search'),
      controller: query,
      textInputAction: TextInputAction.search,
      decoration: InputDecoration(
        hintText: m.t('homeSearchHint'),
        hintMaxLines: 2,
        contentPadding: const EdgeInsets.symmetric(vertical: 12),
        prefixIcon: PopupMenuButton<String>(
          key: const ValueKey('home-search-kind'),
          tooltip: m.t('searchMode'),
          initialValue: kind,
          onSelected: onKind,
          itemBuilder: (_) => [
            PopupMenuItem(
              value: 'vendor',
              child: Text(m.t('navManufacturers')),
            ),
            PopupMenuItem(value: 'product', child: Text(m.t('navProducts'))),
          ],
          child: Padding(
            padding: const EdgeInsets.only(left: 12, right: 4),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(
                  m.t(
                    kind == 'vendor'
                        ? 'searchVendorShort'
                        : 'searchProductShort',
                  ),
                  style: const TextStyle(fontSize: 13),
                ),
                const Icon(Icons.arrow_drop_down, size: 18),
              ],
            ),
          ),
        ),
        suffixIcon: IconButton(
          tooltip: m.t('homeSearchAction'),
          onPressed: onSearch,
          icon: const Icon(Icons.search),
        ),
      ),
      onSubmitted: (_) => onSearch(),
    );
  }
}

class HomeBrandBanner extends StatelessWidget {
  final int? mediaID;
  const HomeBrandBanner({super.key, this.mediaID});
  @override
  Widget build(BuildContext context) => mediaID == null || mediaID! <= 0
      ? const SizedBox.shrink(key: ValueKey('home-banner'))
      : Semantics(
          button: true,
          label: syncText(context, '查看完整图片', 'View full image'),
          child: Material(
            key: const ValueKey('home-banner'),
            color: industrialSurface,
            borderRadius: BorderRadius.circular(12),
            clipBehavior: Clip.antiAlias,
            child: InkWell(
              onTap: () => push(
                context,
                PhotoViewer(
                  ids: [mediaID!],
                  initial: 0,
                  title: syncText(context, '图片预览', 'Image preview'),
                ),
              ),
              child: Photo(
                key: const ValueKey('home-landmark-photo'),
                id: mediaID!,
                thumb: false,
                aspectRatio: 2.5,
                fit: BoxFit.cover,
                alignment: Alignment.topCenter,
                radius: 0,
              ),
            ),
          ),
        );
}

class HomeQuickActions extends StatelessWidget {
  const HomeQuickActions({super.key});
  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.only(top: 8),
    child: LayoutBuilder(
      builder: (context, c) {
        final buttons = [
          OutlinedButton.icon(
            key: const ValueKey('home-map'),
            onPressed: () => openManufacturerMap(context),
            icon: const Icon(Icons.map_outlined),
            label: Text(syncText(context, '厂商地图', 'Manufacturer map')),
          ),
          FilledButton.icon(
            key: const ValueKey('home-admission'),
            onPressed: () => openManufacturerEntry(context),
            icon: const Icon(Icons.add_business_outlined),
            label: Text(syncText(context, '免费入驻', 'Join free')),
          ),
        ];
        if (MediaQuery.textScalerOf(context).scale(1) >= 1.5 ||
            c.maxWidth < 300) {
          return Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [buttons[0], space(8), buttons[1]],
          );
        }
        return Row(
          children: [
            Expanded(child: buttons[0]),
            const SizedBox(width: 8),
            Expanded(child: buttons[1]),
          ],
        );
      },
    ),
  );
}

class HomeLoadState extends StatelessWidget {
  final Object? error;
  final VoidCallback? onRetry;
  const HomeLoadState({super.key, this.error, this.onRetry});
  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    if (error == null) {
      return const Padding(
        padding: EdgeInsets.all(24),
        child: Center(child: CircularProgressIndicator()),
      );
    }
    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        Text(
          m.t(
            error is ApiFailure ? (error as ApiFailure).code : 'network_error',
          ),
        ),
        TextButton(onPressed: onRetry, child: Text(m.t('retry'))),
      ],
    );
  }
}

void openManufacturerEntry(BuildContext context) {
  final m = ModelScope.of(context);
  if (isMerchant(m)) {
    push(context, const MerchantScreen());
  } else {
    unawaited(authenticateManufacturerEntry(context));
  }
}

class HomeJoinCard extends StatelessWidget {
  const HomeJoinCard({super.key});
  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: const Color(0xffeaf2ff),
        borderRadius: BorderRadius.circular(14),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            m.t('homeJoinTitle'),
            style: Theme.of(context).textTheme.titleMedium,
          ),
          space(4),
          Text(
            m.t('homeJoinSub'),
            style: Theme.of(context).textTheme.bodyMedium,
          ),
          space(12),
          FilledButton(
            onPressed: () => openManufacturerEntry(context),
            child: Text(m.t('homeJoinAction')),
          ),
        ],
      ),
    );
  }
}
