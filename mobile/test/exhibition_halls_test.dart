import 'dart:async';
import 'dart:convert';
import 'package:dalunongji/main.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'app_navigation_test.dart' as nav;

void viewport(WidgetTester tester, double width) {
  tester.view.physicalSize = Size(width, 844);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);
}

Future<void> chooseHall(WidgetTester tester, ExhibitionHall hall) async {
  await nav.tap(tester, find.byKey(const ValueKey('home-hall-selector')));
  await nav.tap(tester, find.byKey(ValueKey('hall-option-${hall.name}')));
}

void main() {
  testWidgets(
    'Brand and hall share a row; home has no category entries; preview never queries business data',
    (tester) async {
      viewport(tester, 390);
      final calls = <Uri>[];
      final m = await nav.fixture(signed: false, calls: calls);
      await tester.pumpWidget(nav.harness(m));
      await tester.pumpAndSettle();
      expect(
        tester.getCenter(find.text('农业机械厅')).dy,
        closeTo(tester.getCenter(find.text('中企展馆')).dy, 5),
      );
      expect(find.byType(ChoiceChip), findsNothing);
      expect(find.text('农业机械'), findsNothing);
      expect(find.text('农机配件'), findsNothing);
      expect(find.text('液压油缸'), findsNothing);
      expect(find.text('齿轮'), findsNothing);
      expect(find.text('耕整地机械'), findsNothing);
      final before = calls.length;
      await chooseHall(tester, ExhibitionHall.cablesPreview);
      expect(m.activeHall, ExhibitionHall.cablesPreview);
      for (final tab in [0, 1, 2]) {
        await nav.tap(tester, find.byKey(ValueKey('app-tab-$tab')));
        expect(find.byKey(const ValueKey('hall-coming-soon')), findsOneWidget);
        expect(find.byType(NativeVendorCard), findsNothing);
        expect(find.byType(ItemTile), findsNothing);
        expect(find.byType(TextField), findsNothing);
        expect(find.byType(Photo), findsNothing);
      }
      expect(calls.length, before);
      await nav.tap(tester, find.byKey(const ValueKey('app-tab-3')));
      await nav.tap(tester, find.byKey(const ValueKey('my-favorites')));
      expect(find.byType(FavoritesScreen), findsOneWidget);
      await tester.tap(find.byType(BackButton).first);
      await tester.pumpAndSettle();
      await nav.tap(tester, find.byKey(const ValueKey('app-tab-0')));
      await chooseHall(tester, ExhibitionHall.agriculture);
      expect(find.byType(IndustrialHomeScreen), findsOneWidget);
      expect(find.byType(HomeBrandBanner), findsOneWidget);
      expect(calls.where((u) => u.path.endsWith('/home')), hasLength(1));
      expect(tester.takeException(), isNull);
      await tester.pumpWidget(const SizedBox());
    },
  );

  testWidgets(
    'Preview can initialize without a public home request and opening directories stays lazy',
    (tester) async {
      viewport(tester, 390);
      final calls = <Uri>[];
      final m = await nav.fixture(signed: false, calls: calls);
      m.selectHall(ExhibitionHall.cablesPreview);
      await tester.pumpWidget(nav.harness(m));
      await tester.pumpAndSettle();
      for (final tab in [1, 2, 0]) {
        await nav.tap(tester, find.byKey(ValueKey('app-tab-$tab')));
      }
      expect(calls.where((u) => !u.path.endsWith('/halls')), isEmpty);
      await nav.tap(tester, find.byKey(const ValueKey('return-agriculture')));
      expect(
        calls.where((u) => !u.path.endsWith('/halls')).map((u) => u.path),
        unorderedEquals(['/api/v1/public/home', '/api/v1/public/taxonomy']),
      );
      expect(m.activeHall, ExhibitionHall.agriculture);
      await tester.pumpWidget(const SizedBox());
    },
  );

  testWidgets(
    'Returning to agriculture retains directory query, category, page and scroll',
    (tester) async {
      viewport(tester, 390);
      final calls = <Uri>[];
      final m = await nav.fixture(signed: false, calls: calls);
      await tester.pumpWidget(nav.harness(m));
      await tester.pumpAndSettle();
      await nav.tap(tester, find.byKey(const ValueKey('app-tab-1')));
      await tester.enterText(find.byType(TextField).first, '440');
      await tester.testTextInput.receiveAction(TextInputAction.search);
      await tester.pumpAndSettle();
      await nav.tap(tester, find.byType(DropdownButtonFormField<int>).first);
      await nav.tap(tester, find.text('农业机械').last);
      await nav.tap(tester, find.text('下一页'));
      final controller = tester
          .widget<CustomScrollView>(find.byType(CustomScrollView).last)
          .controller!;
      final offset = controller.offset;
      final vendorReads = calls
          .where((u) => u.path.endsWith('/vendors'))
          .length;
      await nav.tap(tester, find.byKey(const ValueKey('app-tab-0')));
      await chooseHall(tester, ExhibitionHall.cablesPreview);
      await nav.tap(tester, find.byKey(const ValueKey('app-tab-1')));
      await chooseHall(tester, ExhibitionHall.agriculture);
      expect(
        tester.widget<TextField>(find.byType(TextField).first).controller!.text,
        '440',
      );
      expect(find.text('农业机械'), findsOneWidget);
      expect(find.textContaining('2 /'), findsOneWidget);
      expect(controller.offset, offset);
      expect(
        calls.where((u) => u.path.endsWith('/vendors')).length,
        vendorReads,
      );
      expect(tester.takeException(), isNull);
      await tester.pumpWidget(const SizedBox());
    },
  );

  testWidgets(
    'A late agricultural response cannot appear in the preview hall',
    (tester) async {
      viewport(tester, 390);
      final pending = Completer<http.Response?>();
      final m = await nav.fixture(
        signed: false,
        intercept: (r) =>
            r.url.path.endsWith('/home') ? pending.future : Future.value(null),
      );
      await tester.pumpWidget(nav.harness(m));
      await tester.pump();
      m.selectHall(ExhibitionHall.cablesPreview);
      await tester.pumpAndSettle();
      pending.complete(
        http.Response(
          jsonEncode({
            'vendors': [nav.company()],
            'products': [nav.product()],
            'hero': {'media_id': 22},
          }),
          200,
          headers: {'content-type': 'application/json; charset=utf-8'},
        ),
      );
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('hall-coming-soon')), findsOneWidget);
      expect(find.byType(NativeVendorCard), findsNothing);
      expect(find.byType(Photo), findsNothing);
      await nav.tap(tester, find.byKey(const ValueKey('return-agriculture')));
      expect(find.byKey(const ValueKey('home-landmark-photo')), findsOneWidget);
      expect(tester.takeException(), isNull);
      await tester.pumpWidget(const SizedBox());
    },
  );

  testWidgets(
    'Banner reuses and refreshes backend image ID, with branded error and absent-image fallbacks',
    (tester) async {
      viewport(tester, 390);
      var mediaID = 22;
      var reads = 0;
      final m = await nav.fixture(
        signed: false,
        intercept: (r) async {
          if (!r.url.path.endsWith('/home')) return null;
          reads++;
          return http.Response(
            jsonEncode({
              'vendors': [],
              'products': [],
              'hero': {'media_id': mediaID},
            }),
            200,
          );
        },
      );
      await tester.pumpWidget(nav.harness(m));
      await tester.pumpAndSettle();
      Photo landmark() => tester.widget<Photo>(
        find.byKey(const ValueKey('home-landmark-photo')),
      );
      expect(landmark().id, 22);
      expect(landmark().alignment, Alignment.topCenter);
      expect(landmark().thumb, isFalse);
      expect(find.byKey(const ValueKey('landmark-fallback')), findsWidgets);
      mediaID = 77;
      await tester.drag(
        find.byType(CustomScrollView).first,
        const Offset(0, 500),
      );
      await tester.pumpAndSettle();
      expect(landmark().id, 77);
      expect(reads, 2);
      mediaID = 0;
      await tester.drag(
        find.byType(CustomScrollView).first,
        const Offset(0, 500),
      );
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('home-landmark-photo')), findsNothing);
      expect(find.byKey(const ValueKey('landmark-fallback')), findsOneWidget);
      expect(find.text(m.t('homeHero')), findsOneWidget);
      await tester.pumpWidget(const SizedBox());
    },
  );

  for (final width in [360.0, 390.0, 412.0]) {
    for (final lang in ['zh', 'en']) {
      for (final scale in [1.0, 1.4, 2.0]) {
        testWidgets('Hall menu and preview fit $width $lang scale $scale', (
          tester,
        ) async {
          viewport(tester, width);
          final m = await nav.fixture(signed: false);
          m.language = lang;
          await tester.pumpWidget(nav.harness(m, scale: scale));
          await tester.pumpAndSettle();
          await chooseHall(tester, ExhibitionHall.cablesPreview);
          for (final tab in [0, 1, 2, 3]) {
            await nav.tap(tester, find.byKey(ValueKey('app-tab-$tab')));
            expect(tester.takeException(), isNull);
          }
          await tester.pumpWidget(const SizedBox());
        });
      }
    }
  }
}
