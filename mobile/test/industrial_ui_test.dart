import 'dart:convert';
import 'dart:io';
import 'dart:ui' as ui;
import 'package:dalunongji/main.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  Future<AppModel> model({http.Client? client}) async {
    SharedPreferences.setMockInitialValues({'base_url': 'http://fixture'});
    return AppModel(
      await SharedPreferences.getInstance(),
      {
        for (final lang in ['zh', 'en'])
          lang: asMap(
            jsonDecode(File('assets/i18n/$lang.json').readAsStringSync()),
          ),
      },
      client:
          client ??
          MockClient((_) async => http.Response('{"items":[],"total":0}', 200)),
    );
  }

  Widget app(
    AppModel m,
    Widget home, {
    double scale = 1,
    Key? key,
    String? font,
  }) => RepaintBoundary(
    key: key,
    child: ModelScope(
      model: m,
      child: MaterialApp(
        debugShowCheckedModeBanner: false,
        theme: industrialTheme().copyWith(
          textTheme: industrialTheme().textTheme.apply(fontFamily: font),
        ),
        builder: (context, child) => MediaQuery(
          data: MediaQuery.of(
            context,
          ).copyWith(textScaler: TextScaler.linear(scale)),
          child: child!,
        ),
        home: home,
      ),
    ),
  );
  testWidgets(
    'Product photos preserve equipment and industrial theme is blue',
    (tester) async {
      final m = await model();
      await tester.pumpWidget(app(m, const Scaffold(body: Photo(id: 999))));
      await tester.pumpAndSettle();
      expect(tester.widget<Image>(find.byType(Image)).fit, BoxFit.contain);
      expect(industrialTheme().colorScheme.primary, industrialBlue);
      expect(find.byIcon(Icons.image_not_supported_outlined), findsOneWidget);
    },
  );
  testWidgets(
    'Product browsing reuses the public endpoint and keeps keyword on return',
    (tester) async {
      final calls = <Uri>[];
      final m = await model(
        client: MockClient((r) async {
          calls.add(r.url);
          if (r.url.path.endsWith('taxonomy')) {
            return http.Response('{"categories":[],"regions":[]}', 200);
          }
          return http.Response('{"items":[],"total":0}', 200);
        }),
      );
      await tester.pumpWidget(
        app(m, const BrowseScreen(kind: 'product', query: '440')),
      );
      await tester.pumpAndSettle();
      expect(
        calls.any(
          (u) =>
              u.path.endsWith('/public/products') &&
              u.queryParameters['q'] == '440',
        ),
        isTrue,
      );
      expect(find.text('产品'), findsOneWidget);
      expect(find.text('440'), findsOneWidget);
      expect(tester.takeException(), isNull);
    },
  );
  testWidgets(
    'Brand hero and exhibit cards adapt to narrow screens and large text',
    (tester) async {
      tester.view.physicalSize = const Size(360, 844);
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      final m = await model();
      await tester.pumpWidget(
        app(
          m,
          Scaffold(
            body: SingleChildScrollView(
              child: Column(
                children: [
                  NativeBrandHero(
                    data: {
                      'name': '河北工业制造设备科技有限公司企业独立品牌展厅',
                      'region_name': '河北',
                      'main_products': '工业设备与农业机械',
                      'company_profile': {},
                    },
                    onContact: () {},
                    onShare: () {},
                  ),
                  Padding(
                    padding: const EdgeInsets.all(20),
                    child: ExhibitGrid(
                      items: [
                        for (var id = 1; id <= 3; id++)
                          {
                            'id': id,
                            'kind': 'product',
                            'data': {'name': '工业展品 TEST-$id', 'media_ids': []},
                          },
                      ],
                    ),
                  ),
                ],
              ),
            ),
          ),
          scale: 1.4,
        ),
      );
      await tester.pumpAndSettle();
      expect(tester.takeException(), isNull);
      final cards = tester.widgetList<ItemTile>(find.byType(ItemTile));
      expect(cards.every((c) => !c.compact), isTrue);
      expect(find.text('企业独立品牌展厅'), findsOneWidget);
    },
  );
  testWidgets(
    'Render native UI screenshot evidence from published company data',
    (tester) async {
      final fixtures = asMap(
        jsonDecode(File('../.local/ui-native-fixture.json').readAsStringSync()),
      );
      final fontFile = File('C:/Windows/Fonts/msyh.ttc');
      String? font;
      await tester.runAsync(() async {
        final iconLoader = FontLoader('MaterialIcons');
        iconLoader.addFont(
          Future.value(
            ByteData.sublistView(
              File(
                '../.tools/flutter/bin/cache/artifacts/material_fonts/MaterialIcons-Regular.otf',
              ).readAsBytesSync(),
            ),
          ),
        );
        await iconLoader.load();
        if (fontFile.existsSync()) {
          final loader = FontLoader('UiChinese');
          loader.addFont(
            Future.value(ByteData.sublistView(fontFile.readAsBytesSync())),
          );
          await loader.load();
          font = 'UiChinese';
        }
        final needed = <int>{1};
        for (final item in [
          ...rows(fixtures['home']['products']),
          ...rows(fixtures['products']['items']),
        ]) {
          final ids = imageIDs(asMap(item['data']));
          if (ids.isNotEmpty) needed.add(ids.first);
        }
        final cache = PaintingBinding.instance.imageCache;
        final originalLimit = cache.maximumSizeBytes;
        cache.maximumSizeBytes = 512 * 1024 * 1024;
        addTearDown(() => cache.maximumSizeBytes = originalLimit);
        for (final file in Directory(
          '../.local/ui-native-images',
        ).listSync().whereType<File>()) {
          final id = file.uri.pathSegments.last.split('.').first;
          if (!needed.contains(int.parse(id))) continue;
          final codec = await ui.instantiateImageCodec(
            file.readAsBytesSync(),
            targetWidth: 800,
          );
          final frame = await codec.getNextFrame();
          for (final suffix in ['', '?thumb=1']) {
            final provider = NetworkImage(
              'http://fixture/api/v1/media/$id$suffix',
            );
            PaintingBinding.instance.imageCache.putIfAbsent(
              provider,
              () => OneFrameImageStreamCompleter(
                Future.value(ImageInfo(image: frame.image.clone())),
              ),
            );
          }
          frame.image.dispose();
          codec.dispose();
        }
      });
      final baseModel = await model();
      final m = _ScreenshotModel(
        baseModel.prefs,
        baseModel.resources,
        fixtures,
      );
      tester.view.physicalSize = const Size(390, 844);
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      Future<void> capture(
        String name,
        Widget widget, {
        double scale = 1,
      }) async {
        final key = GlobalKey();
        await tester.pumpWidget(
          app(m, widget, key: key, font: font, scale: scale),
        );
        await tester.pumpAndSettle();
        expect(tester.takeException(), isNull);
        await expectLater(
          find.byKey(key),
          matchesGoldenFile('../../docs/screenshots/$name.png'),
        );
      }

      await capture('onboard-app-form', const EditorScreen(kind: 'vendor'));
      await capture(
        'onboard-app-large-text',
        const EditorScreen(kind: 'vendor'),
        scale: 1.4,
      );
      await capture(
        'onboard-app-brand-preview',
        Scaffold(
          appBar: AppBar(title: const Text('品牌名片效果预览')),
          body: SingleChildScrollView(
            padding: const EdgeInsets.all(20),
            child: NativeBrandPreview(
              data: asMap(fixtures['showroom']['item']['data']),
            ),
          ),
        ),
      );
      await capture('ui-app-home', const HomeScreen());
      await capture(
        'ui-app-showroom',
        const CompanyShowroomScreen(slug: 'jinong'),
      );
      await tester.drag(find.byType(NestedScrollView), const Offset(0, -500));
      await tester.pumpAndSettle();
      expect(find.byType(ChoiceChip), findsNWidgets(5));
      await expectLater(
        find.byType(RepaintBoundary).first,
        matchesGoldenFile('../../docs/screenshots/ui-app-exhibits.png'),
      );
      await capture('ui-app-products', const BrowseScreen(kind: 'product'));
      await capture(
        'ui-app-large-text',
        const CompanyShowroomScreen(slug: 'jinong'),
        scale: 1.4,
      );
      await tester.pumpWidget(const SizedBox());
    },
    skip: Platform.environment['RUN_UI_SCREENSHOTS'] != '1',
  );
}

class _ScreenshotModel extends AppModel {
  final Json fixtures;
  _ScreenshotModel(super.prefs, super.resources, this.fixtures);
  @override
  Future<dynamic> api(String path, {String method = "GET", Json? data}) async {
    if (path.startsWith("/public/home")) return fixtures["home"];
    if (path.startsWith("/public/showrooms/")) return fixtures["showroom"];
    if (path.startsWith("/public/taxonomy")) return fixtures["taxonomy"];
    if (path.startsWith("/public/products?")) {
      final result = Map<String, dynamic>.from(fixtures["products"]);
      if (path.contains("page_size=3&")) {
        result["items"] = rows(result["items"]).take(3).toList();
      }
      return result;
    }
    return {"items": [], "total": 0};
  }
}
