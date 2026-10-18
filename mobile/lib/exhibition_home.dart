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
        m.publicTaxonomy().then<void>(
          (_) {},
          onError: (Object e, StackTrace s) {},
        ),
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
                    space(12),
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
                  final vendors = allVendors.take(4).toList();
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
                        SliverList.builder(
                          itemCount: vendors.length,
                          itemBuilder: (context, index) {
                            final vendor = vendors[index];
                            return NativeVendorCard(
                              key: ValueKey('home-vendor-${vendor['id']}'),
                              item: vendor,
                              productImages: rows(data['products'])
                                  .where((p) => p['vendor_id'] == vendor['id'])
                                  .toList(),
                            );
                          },
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
                      const HomeJoinCard(),
                      space(20),
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
      title: Row(
        children: [
          Flexible(
            flex: 4,
            child: Tooltip(
              message: m.t('brand'),
              child: Text(
                m.t('brand'),
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: const TextStyle(
                  fontSize: 20,
                  fontWeight: FontWeight.w700,
                ),
              ),
            ),
          ),
          const SizedBox(width: 8),
          Flexible(
            flex: 5,
            child: PopupMenuButton<ExhibitionHall>(
              key: const ValueKey('home-hall-selector'),
              tooltip: m.t('chooseHall'),
              initialValue: m.activeHall,
              onSelected: m.selectHall,
              itemBuilder: (_) => [
                for (final hall in m.availableHalls)
                  PopupMenuItem(
                    key: ValueKey('hall-option-${hall.name}'),
                    value: hall,
                    child: Padding(
                      padding: const EdgeInsets.symmetric(vertical: 6),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          Text(exhibitionHallName(m, hall)),
                          if (!hall.isOpen)
                            Text(
                              m.t('hallPreparing'),
                              style: const TextStyle(
                                fontSize: 12,
                                color: Color(0xff66768a),
                              ),
                            ),
                        ],
                      ),
                    ),
                  ),
              ],
              child: ConstrainedBox(
                constraints: const BoxConstraints(minHeight: 48),
                child: Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Flexible(
                      child: Text(
                        exhibitionHallName(m, m.activeHall),
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: const TextStyle(
                          fontSize: 13,
                          color: industrialInk,
                        ),
                      ),
                    ),
                    const Icon(Icons.arrow_drop_down, size: 18),
                  ],
                ),
              ),
            ),
          ),
        ],
      ),
      actions: [
        TextButton(
          style: TextButton.styleFrom(
            minimumSize: const Size(44, 44),
            padding: const EdgeInsets.symmetric(horizontal: 8),
          ),
          onPressed: m.toggleLanguage,
          child: Text(m.language == 'zh' ? 'English' : '中文'),
        ),
        IconButton(
          key: const ValueKey('home-profile'),
          tooltip: m.t('navMine'),
          onPressed: () {
            final navigation = AppNavigation.of(context);
            if (navigation != null) {
              navigation.openMine();
            } else {
              push(context, const PersonalScreen());
            }
          },
          icon: const Icon(Icons.person_outline),
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
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    final family = Theme.of(context).textTheme.bodyMedium?.fontFamily;
    final titleStyle = TextStyle(
      fontFamily: family,
      fontSize: 20,
      height: 1.25,
      color: Colors.white,
      fontWeight: FontWeight.w700,
    );
    final subtitleStyle = TextStyle(
      fontFamily: family,
      fontSize: 12,
      height: 1.4,
      color: const Color(0xffe0ebf8),
    );
    const fallback = DecoratedBox(
      key: ValueKey('landmark-fallback'),
      decoration: BoxDecoration(
        gradient: LinearGradient(
          colors: [industrialNavy, Color(0xff124d89)],
          begin: Alignment.topLeft,
          end: Alignment.bottomRight,
        ),
      ),
    );
    return LayoutBuilder(
      builder: (context, c) {
        final imageHeight = (c.maxWidth * .58).clamp(200.0, 220.0);
        double textHeight(String text, TextStyle style) {
          final painter = TextPainter(
            text: TextSpan(text: text, style: style),
            textDirection: Directionality.of(context),
            textScaler: MediaQuery.textScalerOf(context),
            locale: Localizations.localeOf(context),
          )..layout(maxWidth: max(1, c.maxWidth - 32));
          final height = painter.height;
          painter.dispose();
          return height;
        }

        final copyHeight =
            textHeight(
              '${m.t('brand')} · ${exhibitionHallName(m, m.activeHall)}',
              titleStyle,
            ) +
            6 +
            textHeight(m.t('homeHeroSub'), subtitleStyle);
        final height = max(imageHeight, 128 + copyHeight + 44);
        final clearStop = (height - copyHeight - 44) / height;
        final textStop = (height - copyHeight - 16) / height;
        return Container(
          key: const ValueKey('home-banner'),
          height: height,
          decoration: BoxDecoration(
            color: industrialNavy,
            borderRadius: BorderRadius.circular(16),
          ),
          clipBehavior: Clip.antiAlias,
          child: Stack(
            children: [
              const Positioned.fill(child: fallback),
              if (mediaID != null && mediaID! > 0)
                Positioned(
                  top: 0,
                  left: 0,
                  right: 0,
                  height: imageHeight,
                  child: Semantics(
                    image: true,
                    label: m.t('landmarkScene'),
                    child: ExcludeSemantics(
                      child: Photo(
                        key: const ValueKey('home-landmark-photo'),
                        id: mediaID!,
                        thumb: false,
                        height: imageHeight,
                        fit: BoxFit.cover,
                        alignment: Alignment.topCenter,
                        radius: 0,
                        loadingPlaceholder: fallback,
                        errorPlaceholder: fallback,
                      ),
                    ),
                  ),
                ),
              Positioned.fill(
                child: DecoratedBox(
                  decoration: BoxDecoration(
                    gradient: LinearGradient(
                      begin: Alignment.topCenter,
                      end: Alignment.bottomCenter,
                      colors: const [
                        Colors.transparent,
                        Colors.transparent,
                        Color(0xed071b36),
                        industrialNavy,
                      ],
                      stops: [0, clearStop, textStop, 1],
                    ),
                  ),
                ),
              ),
              Positioned(
                left: 16,
                right: 16,
                bottom: 16,
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      '${m.t('brand')} · ${exhibitionHallName(m, m.activeHall)}',
                      style: titleStyle,
                    ),
                    const SizedBox(height: 6),
                    Text(m.t('homeHeroSub'), style: subtitleStyle),
                  ],
                ),
              ),
            ],
          ),
        );
      },
    );
  }
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
  if (m.token.isNotEmpty && m.account['role'] != 'visitor') {
    returnToMerchant(context);
  } else if (m.account['role'] == 'visitor') {
    unawaited(loginManufacturer(context));
  } else {
    push(context, const EditorScreen(kind: 'vendor'));
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
