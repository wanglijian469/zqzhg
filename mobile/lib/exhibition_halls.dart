part of 'main.dart';

/// Preview destinations are presentation-only: they have no backend hall ID.
/// A second live hall requires hall-scoped public queries before activation.
enum ExhibitionHall { agriculture, cablesPreview }

String exhibitionHallName(AppModel m, ExhibitionHall hall) =>
    m.t(hall == ExhibitionHall.agriculture ? 'homeHall' : 'cablesHall');

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
                  m.t('hallComingSoonDescription'),
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
