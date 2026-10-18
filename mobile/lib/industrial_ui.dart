part of 'main.dart';

const industrialNavy = Color(0xff071b36);
const industrialBlue = Color(0xff1769d2);
const industrialSurface = Color(0xfff5f7fa);
const industrialLine = Color(0xffe5eaf1);
const industrialInk = Color(0xff172b42);

ThemeData industrialTheme() => ThemeData(
  useMaterial3: true,
  colorScheme: ColorScheme.fromSeed(
    seedColor: industrialBlue,
    primary: industrialBlue,
    surface: Colors.white,
  ),
  scaffoldBackgroundColor: industrialSurface,
  textTheme: const TextTheme(
    headlineLarge: TextStyle(
      fontSize: 32,
      height: 1.3,
      fontWeight: FontWeight.w700,
      color: industrialInk,
    ),
    headlineMedium: TextStyle(
      fontSize: 26,
      height: 1.4,
      fontWeight: FontWeight.w700,
      color: industrialInk,
    ),
    titleLarge: TextStyle(
      fontSize: 21,
      height: 1.4,
      fontWeight: FontWeight.w700,
      color: industrialInk,
    ),
    titleMedium: TextStyle(
      fontSize: 17,
      height: 1.5,
      fontWeight: FontWeight.w600,
      color: industrialInk,
    ),
    bodyLarge: TextStyle(fontSize: 16, height: 1.6, color: industrialInk),
    bodyMedium: TextStyle(fontSize: 14, height: 1.6, color: industrialInk),
    bodySmall: TextStyle(fontSize: 12, height: 1.5, color: Color(0xff67758a)),
  ),
  appBarTheme: const AppBarTheme(
    backgroundColor: Colors.white,
    foregroundColor: industrialInk,
    surfaceTintColor: Colors.transparent,
    scrolledUnderElevation: 1,
    elevation: 0,
  ),
  cardTheme: CardThemeData(
    color: Colors.white,
    surfaceTintColor: Colors.transparent,
    elevation: 0,
    margin: const EdgeInsets.symmetric(vertical: 8),
    shape: RoundedRectangleBorder(
      borderRadius: BorderRadius.circular(14),
      side: const BorderSide(color: industrialLine),
    ),
  ),
  inputDecorationTheme: InputDecorationTheme(
    filled: true,
    fillColor: Colors.white,
    border: OutlineInputBorder(
      borderRadius: BorderRadius.circular(10),
      borderSide: const BorderSide(color: industrialLine),
    ),
    enabledBorder: OutlineInputBorder(
      borderRadius: BorderRadius.circular(10),
      borderSide: const BorderSide(color: industrialLine),
    ),
    contentPadding: const EdgeInsets.symmetric(horizontal: 16, vertical: 14),
  ),
  filledButtonTheme: FilledButtonThemeData(
    style: FilledButton.styleFrom(
      backgroundColor: industrialBlue,
      foregroundColor: Colors.white,
      minimumSize: const Size(48, 48),
      padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 14),
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
    ),
  ),
  outlinedButtonTheme: OutlinedButtonThemeData(
    style: OutlinedButton.styleFrom(
      foregroundColor: industrialInk,
      minimumSize: const Size(48, 48),
      side: const BorderSide(color: industrialLine),
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 14),
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
    ),
  ),
  navigationBarTheme: const NavigationBarThemeData(
    backgroundColor: Colors.white,
    indicatorColor: Color(0xffeaf2ff),
    surfaceTintColor: Colors.transparent,
  ),
  dividerTheme: const DividerThemeData(color: industrialLine, space: 24),
  chipTheme: ChipThemeData(
    backgroundColor: Colors.white,
    selectedColor: const Color(0xffeaf2ff),
    side: const BorderSide(color: industrialLine),
    shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(8)),
  ),
);

class ExhibitGrid extends StatelessWidget {
  final bool showVendor;
  final List<Json> items;
  const ExhibitGrid({super.key, required this.items, this.showVendor = true});
  @override
  Widget build(BuildContext context) => LayoutBuilder(
    builder: (context, c) {
      final scale = MediaQuery.textScalerOf(context).scale(14) / 14;
      final columns = exhibitColumns(c.maxWidth, scale);
      final width = (c.maxWidth - 14 * (columns - 1)) / columns;
      return Wrap(
        spacing: 14,
        runSpacing: 0,
        children: items
            .map(
              (i) => SizedBox(
                width: width,
                child: ItemTile(
                  item: i,
                  compact: columns == 2,
                  showVendor: showVendor,
                ),
              ),
            )
            .toList(),
      );
    },
  );
}

class IndustrialSectionTitle extends StatelessWidget {
  final String title;
  final VoidCallback? onAll;
  const IndustrialSectionTitle({super.key, required this.title, this.onAll});
  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.only(top: 22, bottom: 10),
    child: Row(
      children: [
        Expanded(
          child: Text(
            title,
            style: Theme.of(
              context,
            ).textTheme.titleMedium?.copyWith(fontSize: 19),
          ),
        ),
        if (onAll != null)
          TextButton(
            onPressed: onAll,
            child: Text(ModelScope.of(context).t('all')),
          ),
      ],
    ),
  );
}

class NativeBrandHero extends StatelessWidget {
  final Widget? favoriteAction;
  final Json data;
  final VoidCallback onContact, onShare;
  const NativeBrandHero({
    super.key,
    this.favoriteAction,
    required this.data,
    required this.onContact,
    required this.onShare,
  });
  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    final p = asMap(data['company_profile'] ?? {});
    final cover = (p['cover_media_id'] ?? 0) as int;
    final ids = imageIDs(data);
    final image = cover > 0 ? cover : ids.firstOrNull;
    final logo = (p['logo_media_id'] ?? 0) as int;
    return Container(
      color: industrialNavy,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          if (image != null)
            Photo(
              id: image,
              thumb: false,
              aspectRatio: 16 / 9,
              fit: BoxFit.cover,
              radius: 0,
            ),
          Padding(
            padding: const EdgeInsets.all(24),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  m.t('showroomIdentity'),
                  style: const TextStyle(
                    color: Color(0xff8fb9e9),
                    fontSize: 10,
                    letterSpacing: 1,
                  ),
                ),
                space(16),
                Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Container(
                      width: 54,
                      height: 54,
                      decoration: BoxDecoration(
                        color: Colors.white,
                        borderRadius: BorderRadius.circular(10),
                      ),
                      child: logo > 0
                          ? Photo(id: logo, aspectRatio: 1, fit: BoxFit.contain)
                          : const Icon(
                              Icons.business_outlined,
                              color: industrialNavy,
                            ),
                    ),
                    const SizedBox(width: 14),
                    Expanded(
                      child: Text(
                        m.content(data, 'name'),
                        style: const TextStyle(
                          color: Colors.white,
                          fontSize: 24,
                          fontWeight: FontWeight.w700,
                          height: 1.4,
                        ),
                      ),
                    ),
                  ],
                ),
                space(12),
                Text(
                  '${data['region_name'] ?? ''} · ${data['main_products'] ?? ''}',
                  style: const TextStyle(
                    color: Color(0xffb6cde7),
                    height: 1.7,
                    fontSize: 13,
                  ),
                ),
                if (m.content(p, 'summary').isNotEmpty) ...[
                  space(12),
                  Text(
                    m.content(p, 'summary'),
                    style: const TextStyle(
                      color: Color(0xff91accb),
                      height: 1.8,
                      fontSize: 13,
                    ),
                  ),
                ],
                space(20),
                Wrap(
                  spacing: 12,
                  runSpacing: 10,
                  children: [
                    FilledButton.icon(
                      onPressed: onContact,
                      icon: const Icon(Icons.phone_outlined, size: 18),
                      label: Text(m.t('showroomContact')),
                    ),
                    OutlinedButton.icon(
                      onPressed: onShare,
                      style: OutlinedButton.styleFrom(
                        foregroundColor: Colors.white,
                        side: const BorderSide(color: Color(0xff476382)),
                      ),
                      icon: const Icon(Icons.share_outlined, size: 18),
                      label: Text(m.t('shareShowroom')),
                    ),
                    ?favoriteAction,
                  ],
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class ShowroomTabsHeader extends SliverPersistentHeaderDelegate {
  final Widget child;
  final double height;
  ShowroomTabsHeader({required this.child, required this.height});
  @override
  double get minExtent => height;
  @override
  double get maxExtent => height;
  @override
  Widget build(
    BuildContext context,
    double shrinkOffset,
    bool overlapsContent,
  ) => Material(
    color: Colors.white,
    elevation: overlapsContent ? 2 : 0,
    child: Align(alignment: Alignment.centerLeft, child: child),
  );
  @override
  bool shouldRebuild(covariant ShowroomTabsHeader old) =>
      old.child != child || old.height != height;
}

class NativeSignatureExhibits extends StatelessWidget {
  final int vendorID;
  const NativeSignatureExhibits({super.key, required this.vendorID});
  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    return AsyncPane(
      identity: '${m.baseUrl}:signature:$vendorID',
      load: () => m.api('/public/showrooms/v-$vendorID'),
      builder: (response) {
        final items = rows(response['representative_products']);
        if (items.isEmpty) return const SizedBox();
        return Container(
          margin: const EdgeInsets.only(bottom: 24),
          padding: const EdgeInsets.all(16),
          decoration: BoxDecoration(
            color: const Color(0xffeaf1fb),
            border: Border.all(color: industrialLine),
            borderRadius: BorderRadius.circular(14),
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                m.t('representativeProducts'),
                style: Theme.of(context).textTheme.titleLarge,
              ),
              space(6),
              Text(
                m.t('representativeSub'),
                style: Theme.of(context).textTheme.bodySmall,
              ),
              space(12),
              SingleChildScrollView(
                scrollDirection: Axis.horizontal,
                child: Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: items
                      .map(
                        (i) => Padding(
                          padding: const EdgeInsets.only(right: 12),
                          child: SizedBox(
                            width: MediaQuery.sizeOf(context).width < 380
                                ? 210
                                : 230,
                            child: ItemTile(item: i, compact: true),
                          ),
                        ),
                      )
                      .toList(),
                ),
              ),
            ],
          ),
        );
      },
    );
  }
}
