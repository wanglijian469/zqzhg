part of 'main.dart';

class ExhibitionHall {
  final int id;
  final String slug, title, titleEN, status, icon, description, descriptionEN;
  const ExhibitionHall({
    required this.id,
    required this.slug,
    required this.title,
    required this.titleEN,
    required this.status,
    this.icon = 'factory',
    this.description = '',
    this.descriptionEN = '',
  });
  static const agriculture = ExhibitionHall(
    id: 1,
    slug: 'agriculture',
    title: '农业机械厅',
    titleEN: 'Agricultural Machinery Hall',
    status: 'open',
    icon: 'tractor',
  );
  static const cablesPreview = ExhibitionHall(
    id: 2,
    slug: 'cable',
    title: '电线电缆厅',
    titleEN: 'Wire & Cable Hall',
    status: 'preparing',
    icon: 'cable',
  );
  static const values = [agriculture, cablesPreview];
  String get name => slug == 'cable' ? 'cablesPreview' : slug;
  bool get isOpen => status == 'open';
  factory ExhibitionHall.fromJson(Json j) => ExhibitionHall(
    id: (j['id'] as num).toInt(),
    slug: j['slug'],
    title: j['name'],
    titleEN: (j['name_en'] ?? j['name']).toString(),
    status: j['status'],
    icon: (j['icon'] ?? 'factory').toString(),
    description: (j['description'] ?? '').toString(),
    descriptionEN: (j['description_en'] ?? '').toString(),
  );
  @override
  bool operator ==(Object other) =>
      other is ExhibitionHall && other.slug == slug;
  @override
  int get hashCode => slug.hashCode;
}

String exhibitionHallName(AppModel m, ExhibitionHall hall) =>
    m.language == 'en' ? hall.titleEN : hall.title;

class HallRegistry {
  final AppModel model;
  HallRegistry(this.model);
  List<ExhibitionHall> items = ExhibitionHall.values;
  ExhibitionHall? _active;
  bool ready = false, legacy = false;
  String? error, _base;
  Future<void>? _pending;
  int generation = 0;
  String get preference => 'active_hall.${Uri.encodeComponent(model.baseUrl)}';
  ExhibitionHall get active {
    if (_active != null) return _active!;
    final saved = model.prefs.getString(preference);
    return _active =
        items.where((h) => h.slug == saved).firstOrNull ??
        (saved == null
            ? ExhibitionHall.agriculture
            : ExhibitionHall(
                id: 0,
                slug: saved,
                title: saved,
                titleEN: saved,
                status: 'unknown',
              ));
  }

  void select(ExhibitionHall hall) {
    if (active == hall && active.status == hall.status) return;
    _active = hall;
    unawaited(model.prefs.setString(preference, hall.slug));
    if (items.contains(hall) && _pending != null && error == 'not_found') {
      ready = true;
      error = null;
    }
    model.publicChanged();
  }

  void reset() {
    generation++;
    _base = null;
    _pending = null;
    ready = false;
    legacy = false;
    error = null;
    items = ExhibitionHall.values;
    _active = null;
  }

  Future<void> load({bool refresh = false}) {
    if (_base != null && _base != model.baseUrl) reset();
    _base = model.baseUrl;
    if (refresh) {
      generation++;
      _pending = null;
      error = null;
    }
    return _pending ??= _load(model.baseUrl, generation);
  }

  Future<void> _load(String base, int stamp) async {
    final selected = active.slug;
    try {
      final response = await model.api('/public/halls');
      final list = rows(response['items']);
      if (list.isEmpty) throw ApiFailure('invalid_request');
      final next = list.map(ExhibitionHall.fromJson).toList();
      if (base != model.baseUrl || stamp != generation) return;
      items = next;
      legacy = false;
    } on ApiFailure catch (e) {
      if (base != model.baseUrl || stamp != generation) return;
      if (e.status == 404) {
        items = ExhibitionHall.values;
        legacy = true;
      } else {
        error = e.code;
        ready = false;
        model.publicChanged();
        return;
      }
    } catch (_) {
      if (base != model.baseUrl || stamp != generation) return;
      error = 'network_error';
      ready = false;
      model.publicChanged();
      return;
    }
    if (base != model.baseUrl || stamp != generation) return;
    final openHalls = items.where((h) => h.isOpen).toList();
    final desired = openHalls.length == 1
        ? openHalls.single.slug
        : _active?.slug ?? selected;
    final resolved = items.where((h) => h.slug == desired).firstOrNull;
    if (resolved != null) _active = resolved;
    if (resolved == null) {
      error = 'not_found';
      ready = false;
    } else {
      error = null;
      ready = true;
    }
    model.publicChanged();
  }
}

class HallLoadScreen extends StatelessWidget {
  const HallLoadScreen({super.key});
  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    return Scaffold(
      appBar: const HomeBrandBar(),
      body: Center(
        child: HomeLoadState(
          error: m.hallError == null ? null : ApiFailure(m.hallError!),
          onRetry: () => m.loadHalls(refresh: true),
        ),
      ),
    );
  }
}

class HallComingSoonScreen extends StatelessWidget {
  const HallComingSoonScreen({super.key});
  @override
  Widget build(BuildContext context) =>
      const Scaffold(appBar: HomeBrandBar(), body: HallComingSoonBody());
}

class HallComingSoonBody extends StatelessWidget {
  const HallComingSoonBody({super.key});
  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    return SafeArea(
      top: false,
      bottom: false,
      child: ListView(
        key: const PageStorageKey('cables-hall-preview'),
        padding: const EdgeInsets.all(16),
        children: [
          Container(
            key: const ValueKey('hall-coming-soon'),
            padding: const EdgeInsets.all(24),
            decoration: BoxDecoration(
              gradient: const LinearGradient(
                colors: [industrialNavy, Color(0xff124d89)],
              ),
              borderRadius: BorderRadius.circular(16),
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const Icon(
                  Icons.electrical_services_outlined,
                  color: Colors.white70,
                  size: 48,
                ),
                space(20),
                Text(
                  exhibitionHallName(m, m.activeHall),
                  style: const TextStyle(
                    color: Colors.white,
                    fontSize: 24,
                    height: 1.3,
                    fontWeight: FontWeight.w700,
                  ),
                ),
                space(10),
                Text(
                  m.t('hallComingSoon'),
                  style: const TextStyle(color: Colors.white, fontSize: 18),
                ),
                space(12),
                Text(
                  (m.language == 'en'
                              ? m.activeHall.descriptionEN
                              : m.activeHall.description)
                          .isNotEmpty
                      ? (m.language == 'en'
                            ? m.activeHall.descriptionEN
                            : m.activeHall.description)
                      : m.t('hallComingSoonDescription'),
                  style: const TextStyle(
                    color: Color(0xffd0e1f7),
                    fontSize: 14,
                    height: 1.6,
                  ),
                ),
              ],
            ),
          ),
          space(24),
          OutlinedButton(
            key: const ValueKey('return-agriculture'),
            onPressed: () => m.selectHall(ExhibitionHall.agriculture),
            child: Text(m.t('returnAgriculture')),
          ),
        ],
      ),
    );
  }
}
