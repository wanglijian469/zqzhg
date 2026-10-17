part of 'main.dart';

/// The four public destinations retain their own list and scroll state.
class AppShell extends StatefulWidget {
  const AppShell({super.key});
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
  void didPopNext() => recordPage();
  int selected = 0;
  final initialQueries = <int, String>{};
  final initialCategories = <int, int?>{};
  final visited = <int>{0};
  final vendors = GlobalKey<_BrowseScreenState>();
  final products = GlobalKey<_BrowseScreenState>();
  AppModel? model;
  void homeCallback() => select(0);
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
    final labels = ['navHome', 'navManufacturers', 'navProducts', 'navMine'];
    final icons = [
      Icons.home_outlined,
      Icons.business_outlined,
      Icons.inventory_2_outlined,
      Icons.person_outline,
    ];
    return PopScope(
      canPop: selected == 0,
      onPopInvokedWithResult: (didPop, _) {
        if (!didPop) select(0);
      },
      child: Scaffold(
        body: AppNavigation(
          browse: browse,
          openMine: () => select(3),
          child: IndexedStack(
            index: selected,
            children: [
              const IndustrialHomeScreen(),
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
  const AppNavigation({
    super.key,
    required this.browse,
    required this.openMine,
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
        leading: const Icon(
          Icons.storefront_outlined,
          color: Color(0xff1769c2),
        ),
        title: Text(
          m.t(owned ? 'myBooth' : 'freeEntry'),
          style: const TextStyle(fontWeight: FontWeight.w600),
        ),
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
          actions: [
            TextButton(
              onPressed: m.toggleLanguage,
              child: Text(m.language == 'zh' ? 'English' : '中文'),
            ),
          ],
        ),
        body: SafeArea(
          top: false,
          bottom: false,
          child: ListView(
            padding: const EdgeInsets.all(16),
            children: [
              if (isMerchant(m))
                MerchantVendorPane(
                  builder: (item) => Column(
                    children: [
                      if (item != null) MerchantCompanyCard(item: item),
                      entry(item != null),
                    ],
                  ),
                )
              else ...[
                Card(
                  child: Padding(
                    padding: const EdgeInsets.all(16),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        const Icon(
                          Icons.account_circle_outlined,
                          size: 40,
                          color: Color(0xff1769c2),
                        ),
                        space(8),
                        Text(
                          m.token.isEmpty
                              ? m.t('guestFavorites')
                              : (m.account['login_name'] ??
                                        m.account['username'] ??
                                        m.t('accountMenu'))
                                    .toString(),
                          maxLines: 2,
                          overflow: TextOverflow.ellipsis,
                        ),
                        if (m.account['role'] == 'visitor')
                          Text(m.t('visitorMerchantHelp')),
                        if (m.token.isEmpty)
                          TextButton(
                            onPressed: () =>
                                push(context, const LoginScreen(visitor: true)),
                            child: Text(m.t('loginToSync')),
                          ),
                      ],
                    ),
                  ),
                ),
                entry(false),
              ],
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
              if (!isMerchant(m))
                TextButton(
                  onPressed: () => loginManufacturer(context),
                  child: Text(m.t('manufacturerAccount')),
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
                  leading: const Icon(Icons.dns_outlined),
                  title: Text(m.t('apiEndpoint')),
                  subtitle: Text(m.baseUrl),
                  trailing: const Icon(Icons.chevron_right),
                  onTap: () => showEndpointSettings(context),
                ),
              ),
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

Future<void> showEndpointSettings(BuildContext context) async {
  final m = ModelScope.of(context);
  final controller = TextEditingController(text: m.baseUrl);
  final url = await showDialog<String>(
    context: context,
    builder: (ctx) => AlertDialog(
      title: Text(m.t('apiEndpoint')),
      content: TextField(
        controller: controller,
        keyboardType: TextInputType.url,
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.pop(ctx),
          child: Text(m.t('cancel')),
        ),
        FilledButton(
          onPressed: () => Navigator.pop(ctx, controller.text.trim()),
          child: Text(m.t('save')),
        ),
      ],
    ),
  );
  // Keep the controller alive until the dialog's exit transition completes.
  await Future<void>.delayed(const Duration(milliseconds: 300));
  controller.dispose();
  if (url == null || !context.mounted) return;
  try {
    await m.changeEndpoint(url);
    if (context.mounted) {
      Navigator.of(context).popUntil((route) => route.isFirst);
    }
  } catch (e) {
    if (context.mounted) showError(context, e);
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
