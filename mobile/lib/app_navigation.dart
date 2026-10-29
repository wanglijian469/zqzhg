part of 'main.dart';

/// The four public destinations retain their own list and scroll state.
class AppShell extends StatefulWidget {
  final bool initiallyInHall;
  const AppShell({super.key, this.initiallyInHall = true});
  @override
  State<AppShell> createState() => _AppShellState();
}

class _AppShellState extends State<AppShell>
    with WidgetsBindingObserver, RouteAware {
  PageRoute<dynamic>? analyticsRoute;
  void recordPage() {
    if (model == null ||
        model!.isHallPreview ||
        selected > 2 ||
        analyticsRoute?.isCurrent != true) {
      return;
    }
    model!.analytics.page(
      selected == 0
          ? "/"
          : selected == 1
          ? "/browse"
          : "/browse/products",
    );
  }

  @override
  void didPopNext() {
    recordPage();
    if (mounted) setState(() {});
  }

  bool welcomeShowing = false;
  int selected = 0;
  Future<void> welcome(AppModel m) async {
    if (welcomeShowing ||
        m.account['welcome_pending'] != true ||
        m.token.isEmpty) {
      return;
    }
    welcomeShowing = true;
    final endpoint = m.baseUrl;
    final session = m.token;
    try {
      final fresh = asMap(await m.api('/auth/me'));
      if (!mounted || m.baseUrl != endpoint || m.token != session) {
        welcomeShowing = false;
        return;
      }
      m.account = fresh;
      await m.prefs.setString('account', jsonEncode(fresh));
      if (fresh['welcome_pending'] != true) {
        welcomeShowing = false;
        return;
      }
    } catch (_) {
      welcomeShowing = false;
      return;
    }
    if (!mounted) {
      welcomeShowing = false;
      return;
    }
    String problem = '';
    bool saving = false;
    final choice = await showDialog<String>(
      context: context,
      barrierDismissible: false,
      builder: (dialog) => StatefulBuilder(
        builder: (c, change) {
          Future<void> choose(String v) async {
            if (saving) return;
            change(() => saving = true);
            try {
              await m.api(
                '/auth/me/welcome',
                method: 'POST',
                data: {'choice': v},
              );
              if (m.baseUrl != endpoint) return;
              m.account['welcome_pending'] = false;
              await m.prefs.setString('account', jsonEncode(m.account));
              if (c.mounted) Navigator.pop(c, v);
            } catch (_) {
              if (c.mounted) {
                change(
                  () => problem = syncText(
                    c,
                    '保存未完成，请重试',
                    'Could not save. Retry.',
                  ),
                );
              }
            } finally {
              if (c.mounted) change(() => saving = false);
            }
          }

          return PopScope(
            canPop: false,
            child: AlertDialog(
              title: Text(syncText(c, '欢迎来到中企展馆', 'Welcome to the exhibition')),
              content: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  for (final pair in [
                    [
                      'manufacturer',
                      '我是厂家，免费入驻',
                      'I manufacture products — join free',
                    ],
                    ['products', '我是采购商，寻找产品', 'Find products'],
                    ['browse', '暂时逛逛展馆', 'Explore the exhibition'],
                    ['skip', '跳过', 'Skip'],
                  ])
                    Padding(
                      padding: const EdgeInsets.only(bottom: 10),
                      child: OutlinedButton(
                        onPressed: saving ? null : () => choose(pair[0]),
                        child: Text(syncText(c, pair[1], pair[2])),
                      ),
                    ),
                  if (problem.isNotEmpty) Text(problem),
                ],
              ),
            ),
          );
        },
      ),
    );
    welcomeShowing = false;
    if (!mounted || m.baseUrl != endpoint) return;
    if (choice == 'products') browse('product');
    if (choice == 'manufacturer') await authenticateManufacturerEntry(context);
  }

  late bool insideHall = widget.initiallyInHall;
  final initialQueries = <int, String>{};
  final initialCategories = <int, int?>{};
  final visited = <int>{0};
  final vendors = GlobalKey<_BrowseScreenState>();
  final products = GlobalKey<_BrowseScreenState>();
  AppModel? model;
  void homeCallback() {
    setState(() {
      selected = 0;
      insideHall = true;
    });
  }

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state == AppLifecycleState.resumed && model != null) {
      unawaited(model!.favorites.refresh());
    }
  }

  void merchantCallback() {
    select(3);
    push(context, const MerchantScreen());
  }

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    final next = ModelScope.of(context);
    if (model != next) {
      model?.openMerchant = null;
      model = next;
      next.openMerchant = merchantCallback;
      next.openHome = homeCallback;
      next.favorites.activate();
    }
    if (!next.isHallPreview) visited.add(selected);
    final route = ModalRoute.of(context);
    if (route is PageRoute<dynamic> && analyticsRoute != route) {
      analyticsRoutes.unsubscribe(this);
      analyticsRoute = route;
      analyticsRoutes.subscribe(this, route);
      recordPage();
    }
  }

  @override
  void dispose() {
    if (model?.openMerchant == merchantCallback) model?.openMerchant = null;
    if (model?.openHome == homeCallback) model?.openHome = null;
    analyticsRoutes.unsubscribe(this);
    model?.analytics.dispose();
    WidgetsBinding.instance.removeObserver(this);
    super.dispose();
  }

  void select(int index) {
    final changed = selected != index;
    setState(() {
      selected = index;
      if (!(model?.isHallPreview ?? false) || index == 0 || index == 3) {
        visited.add(index);
      }
    });
    if (changed) recordPage();
  }

  void browse(String kind, {String? query, int? category}) {
    final index = kind == 'product' ? 2 : 1;
    if (model?.isHallPreview ?? false) {
      select(index);
      return;
    }
    final state = (index == 2 ? products : vendors).currentState;
    if (query != null || category != null) {
      if (state != null) {
        state.searchFor(query ?? '', categoryID: category);
      } else {
        initialQueries[index] = query ?? '';
        initialCategories[index] = category;
      }
    }
    select(index);
  }

  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    if (selected == 0 &&
        m.account['welcome_pending'] == true &&
        !welcomeShowing &&
        (ModalRoute.of(context)?.isCurrent ?? true)) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (mounted) unawaited(welcome(m));
      });
    }
    final labels = ['navHome', 'navManufacturers', 'navProducts', 'navMine'];
    final icons = [
      Icons.home_outlined,
      Icons.business_outlined,
      Icons.inventory_2_outlined,
      Icons.person_outline,
    ];
    return PopScope(
      canPop: selected == 0 && (insideHall || !m.showHallSelector),
      onPopInvokedWithResult: (didPop, _) {
        if (!didPop) {
          if (selected == 0 && insideHall && m.showHallSelector) {
            setState(() => insideHall = false);
          } else {
            select(0);
          }
        }
      },
      child: Scaffold(
        body: AppNavigation(
          browse: browse,
          openExhibition: () => setState(() {
            selected = 0;
            insideHall = false;
          }),
          openMine: () => select(3),
          child: IndexedStack(
            index: selected,
            children: [
              insideHall || !m.showHallSelector
                  ? const IndustrialHomeScreen()
                  : ExhibitionPortalScreen(
                      onSelect: (hall) {
                        m.selectHall(hall);
                        setState(() {
                          insideHall = true;
                          selected = 0;
                        });
                      },
                    ),
              if (visited.contains(1))
                BrowseScreen(
                  key: vendors,
                  query: initialQueries[1] ?? '',
                  category: initialCategories[1],
                )
              else if (m.isHallPreview)
                const HallComingSoonScreen()
              else
                const SizedBox(),
              if (visited.contains(2))
                BrowseScreen(
                  key: products,
                  kind: 'product',
                  query: initialQueries[2] ?? '',
                  category: initialCategories[2],
                )
              else if (m.isHallPreview)
                const HallComingSoonScreen()
              else
                const SizedBox(),
              if (visited.contains(3))
                const PersonalScreen()
              else
                const SizedBox(),
            ],
          ),
        ),
        bottomNavigationBar: Material(
          color: Colors.white,
          child: SafeArea(
            top: false,
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                for (var i = 0; i < labels.length; i++)
                  Expanded(
                    child: Semantics(
                      selected: selected == i,
                      button: true,
                      child: InkWell(
                        key: ValueKey('app-tab-$i'),
                        onTap: () => select(i),
                        child: Padding(
                          padding: const EdgeInsets.symmetric(
                            horizontal: 4,
                            vertical: 10,
                          ),
                          child: Column(
                            mainAxisSize: MainAxisSize.min,
                            children: [
                              Container(
                                padding: const EdgeInsets.symmetric(
                                  horizontal: 16,
                                  vertical: 4,
                                ),
                                decoration: BoxDecoration(
                                  color: selected == i
                                      ? const Color(0xffeaf2ff)
                                      : Colors.transparent,
                                  borderRadius: BorderRadius.circular(20),
                                ),
                                child: Icon(
                                  icons[i],
                                  color: selected == i
                                      ? industrialBlue
                                      : industrialInk,
                                ),
                              ),
                              const SizedBox(height: 4),
                              FittedBox(
                                fit: BoxFit.scaleDown,
                                child: Text(
                                  m.t(labels[i]),
                                  maxLines: 1,
                                  textAlign: TextAlign.center,
                                  style: TextStyle(
                                    fontSize: 12,
                                    color: selected == i
                                        ? industrialBlue
                                        : industrialInk,
                                  ),
                                ),
                              ),
                            ],
                          ),
                        ),
                      ),
                    ),
                  ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

class AppNavigation extends InheritedWidget {
  final void Function(String kind, {String? query, int? category}) browse;
  final VoidCallback openMine;
  final VoidCallback? openExhibition;
  const AppNavigation({
    super.key,
    required this.browse,
    required this.openMine,
    this.openExhibition,
    required super.child,
  });
  static AppNavigation? of(BuildContext context) =>
      context.dependOnInheritedWidgetOfExactType<AppNavigation>();
  @override
  bool updateShouldNotify(AppNavigation oldWidget) => false;
}

void returnToMerchant(BuildContext context) {
  final m = ModelScope.of(context);
  if (m.openMerchant != null) {
    Navigator.of(context).popUntil((route) => route.isFirst);
    m.openMerchant!();
  } else {
    Navigator.pushReplacement(
      context,
      MaterialPageRoute(builder: (_) => const MerchantScreen()),
    );
  }
}

class PersonalScreen extends StatelessWidget {
  const PersonalScreen({super.key});
  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    Widget entry(bool owned) => Card(
      child: ListTile(
        key: const ValueKey('my-booth'),
        leading: const Icon(Icons.storefront_outlined, color: industrialBlue),
        title: Text(m.t(owned ? 'myBooth' : 'freeEntry')),
        trailing: const Icon(Icons.chevron_right),
        onTap: () => push(context, const MerchantScreen()),
      ),
    );
    return Theme(
      data: merchantTheme(context),
      child: Scaffold(
        appBar: AppBar(
          title: Text(m.t('navMine')),
          automaticallyImplyLeading: false,
        ),
        body: SafeArea(
          top: false,
          bottom: false,
          child: ListView(
            padding: const EdgeInsets.all(16),
            children: [
              Card(
                child: ListTile(
                  key: const ValueKey('account-summary'),
                  leading: const Icon(
                    Icons.account_circle_outlined,
                    size: 40,
                    color: industrialBlue,
                  ),
                  title: Text(
                    m.token.isEmpty
                        ? syncText(context, '登录／注册', 'Sign in / Register')
                        : (m.account['phone'] ??
                                  m.account['login_name'] ??
                                  m.account['username'] ??
                                  m.t('accountMenu'))
                              .toString(),
                  ),
                  subtitle: m.token.isEmpty
                      ? Text(
                          syncText(
                            context,
                            '登录后同步收藏和企业资料',
                            'Sync favorites and company materials',
                          ),
                        )
                      : null,
                  trailing: m.token.isEmpty
                      ? const Icon(Icons.chevron_right)
                      : null,
                  onTap: m.token.isEmpty
                      ? () => push(context, const LoginScreen(visitor: true))
                      : null,
                ),
              ),
              if (isMerchant(m))
                MerchantVendorPane(builder: (item) => entry(item != null))
              else
                entry(false),
              Card(
                child: Column(
                  children: [
                    ListTile(
                      key: const ValueKey('my-favorites'),
                      leading: const Icon(Icons.star_border_rounded),
                      title: Text(m.t('favorites')),
                      trailing: const Icon(Icons.chevron_right),
                      onTap: () => push(context, const FavoritesScreen()),
                    ),
                    ListTile(
                      key: const ValueKey('my-settings'),
                      leading: const Icon(Icons.settings_outlined),
                      title: Text(m.t('appSettings')),
                      trailing: const Icon(Icons.chevron_right),
                      onTap: () => push(context, const AccountSettingsScreen()),
                    ),
                  ],
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class AccountSettingsScreen extends StatelessWidget {
  const AccountSettingsScreen({super.key});
  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    return Theme(
      data: merchantTheme(context),
      child: Scaffold(
        appBar: appBar(context, m.t('appSettings')),
        body: SafeArea(
          top: false,
          child: ListView(
            padding: const EdgeInsets.all(16),
            children: [
              Card(
                child: ListTile(
                  leading: const Icon(Icons.language),
                  title: Text(syncText(context, '语言', 'Language')),
                  trailing: Text(m.language == 'zh' ? '简体中文' : 'English'),
                  onTap: m.toggleLanguage,
                ),
              ),
              if (m.token.isNotEmpty &&
                  ['merchant', 'visitor'].contains(m.account['role']))
                Card(
                  child: ListTile(
                    leading: const Icon(Icons.phone_android),
                    title: Text(syncText(context, '手机号', 'Mobile number')),
                    subtitle: Text(
                      m.account['phone_verified'] == true
                          ? m.account['phone'].toString()
                          : syncText(context, '尚未绑定', 'Not linked'),
                    ),
                    trailing: m.account['phone_verified'] == true
                        ? const Icon(Icons.verified_outlined)
                        : const Icon(Icons.chevron_right),
                    onTap: m.account['phone_verified'] == true
                        ? null
                        : () => push(context, const LoginScreen(binding: true)),
                  ),
                ),
              const AppAboutCard(),
              const NativeServiceLinks(),
              if (m.token.isNotEmpty)
                OutlinedButton(
                  onPressed: () async {
                    await confirmFavoriteSignOut(context);
                    if (context.mounted && m.token.isEmpty) {
                      Navigator.of(context).popUntil((r) => r.isFirst);
                    }
                  },
                  child: Text(m.t('logout')),
                ),
            ],
          ),
        ),
      ),
    );
  }
}

Future<void> loginManufacturer(BuildContext context) async {
  final m = ModelScope.of(context);
  final signedIn = await Navigator.push<bool>(
    context,
    MaterialPageRoute(builder: (_) => const LoginScreen()),
  );
  if (context.mounted &&
      signedIn == true &&
      m.token.isNotEmpty &&
      m.account['role'] != 'visitor') {
    returnToMerchant(context);
  }
}

class MerchantResourcesScreen extends StatelessWidget {
  final String kind;
  const MerchantResourcesScreen({super.key, required this.kind});
  @override
  Widget build(BuildContext context) => Scaffold(
    appBar: appBar(
      context,
      ModelScope.of(
        context,
      ).t(kind == 'video' ? 'shortVideos' : 'electronicCatalogs'),
    ),
    body: ListView(
      padding: const EdgeInsets.all(20),
      children: [NativeResourceList(kind: kind, management: true)],
    ),
  );
}
