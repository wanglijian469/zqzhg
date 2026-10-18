import 'dart:convert';
import 'dart:io';
import 'dart:ui' as ui;
import 'package:dalunongji/main.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'app_navigation_test.dart' as nav;

void main() {
  testWidgets('Native V1.7 real-data layout matrix and screenshots', (
    tester,
  ) async {
    final live = asMap(
      jsonDecode(File('../.local/app-v17-live.json').readAsStringSync()),
    );
    await tester.runAsync(() async {
      for (final entry in {
        'NavigationQA': 'C:/Windows/Fonts/msyh.ttc',
        'MaterialIcons':
            '../.tools/flutter/bin/cache/artifacts/material_fonts/MaterialIcons-Regular.otf',
      }.entries) {
        final loader = FontLoader(entry.key);
        loader.addFont(
          Future.value(
            ByteData.sublistView(File(entry.value).readAsBytesSync()),
          ),
        );
        await loader.load();
      }
      final cache = PaintingBinding.instance.imageCache;
      cache.maximumSizeBytes = 512 * 1024 * 1024;
      cache.maximumSize = 1500;
      for (final file in Directory(
        '../.local/app-v17-images',
      ).listSync().whereType<File>()) {
        final id = file.uri.pathSegments.last.split('.').first;
        final codec = await ui.instantiateImageCodec(
          file.readAsBytesSync(),
          targetWidth: 600,
        );
        final frame = await codec.getNextFrame();
        for (final suffix in ['?thumb=1', '']) {
          final image = NetworkImage('http://fixture/api/v1/media/$id$suffix');
          cache.putIfAbsent(
            image,
            () => OneFrameImageStreamCompleter(
              Future.value(ImageInfo(image: frame.image.clone())),
            ),
          );
          final resized = ResizeImage(image, width: 800);
          final key = await resized.obtainKey(ImageConfiguration.empty);
          cache.putIfAbsent(
            key,
            () => OneFrameImageStreamCompleter(
              Future.value(ImageInfo(image: frame.image.clone())),
            ),
          );
        }
        frame.image.dispose();
        codec.dispose();
      }
    });
    final report = <Json>[];
    final output = Directory('../docs/screenshots/app-v1.7')
      ..createSync(recursive: true);
    for (final width in [360.0, 375.0, 390.0, 414.0, 768.0]) {
      for (final language in ['zh', 'en']) {
        for (final scale in [1.0, 1.3]) {
          tester.view.physicalSize = Size(width, 812);
          tester.view.devicePixelRatio = 1;
          final m = await nav.fixture(
            signed: false,
            intercept: (r) async {
              final path = r.url.path;
              dynamic data;
              if (path.endsWith('/halls')) {
                data = live['halls'];
              } else if (path.endsWith('/home')) {
                data = live['home'];
              } else if (path.endsWith('/taxonomy')) {
                data = live['taxonomy'];
              } else if (path.contains('/showrooms/')) {
                data = live['showroom'];
              } else if (path.endsWith('/vendors')) {
                data = live['vendors'];
              } else if (path.endsWith('/products')) {
                data = r.url.queryParameters.containsKey('vendor_id')
                    ? live['companyProducts']
                    : live['products'];
              } else if (path.endsWith('/resources')) {
                data = r.url.queryParameters['kind'] == 'video'
                    ? live['video']
                    : live['catalog'];
              } else {
                return null;
              }
              return http.Response(
                jsonEncode(data),
                200,
                headers: {'content-type': 'application/json; charset=utf-8'},
              );
            },
          );
          m.language = language;
          final scenes = <String, Widget>{
            'home': const HomeScreen(),
            'manufacturers': const BrowseScreen(),
            'products': const BrowseScreen(kind: 'product'),
            'showroom': const CompanyShowroomScreen(slug: 'v-21'),
            'about': const CompanyShowroomScreen(slug: 'v-21', initialTab: 0),
            'company-products': const CompanyShowroomScreen(
              slug: 'v-21',
              initialTab: 1,
            ),
            'videos': const CompanyShowroomScreen(slug: 'v-21', initialTab: 2),
            'catalogs': const CompanyShowroomScreen(
              slug: 'v-21',
              initialTab: 3,
            ),
            'contact': const CompanyShowroomScreen(slug: 'v-21', initialTab: 4),
          };
          for (final entry in scenes.entries) {
            debugPrint('Layout: $width $language $scale ${entry.key}');
            final boundary = GlobalKey();
            await tester.pumpWidget(
              nav.harness(
                m,
                home: entry.value,
                scale: scale,
                boundary: boundary,
              ),
            );
            await tester.pumpAndSettle();
            expect(
              tester.takeException(),
              isNull,
              reason: '$width $language $scale ${entry.key}',
            );
            if (entry.key == 'showroom') {
              final view = tester.state<ScrollableState>(
                find
                    .descendant(
                      of: find.byType(CustomScrollView),
                      matching: find.byType(Scrollable),
                    )
                    .first,
              );
              expect(
                view.position.pixels,
                0,
                reason: 'Initial showroom must retain brand header',
              );
            }
            double? productTop;
            if (entry.key == 'company-products' &&
                find.byType(ItemTile).evaluate().isNotEmpty) {
              productTop = tester.getTopLeft(find.byType(ItemTile).first).dy;
              if (width == 375 && language == 'zh' && scale == 1) {
                expect(productTop, lessThanOrEqualTo(430));
              }
            }
            report.add({
              'width': width,
              'language': language,
              'scale': scale,
              'page': entry.key,
              'firstProductTop': productTop,
              'passed': true,
            });
            if (language == 'zh' &&
                scale == 1 &&
                [375.0, 390.0, 768.0].contains(width)) {
              await tester.runAsync(() async {
                final image =
                    await (boundary.currentContext!.findRenderObject()
                            as RenderRepaintBoundary)
                        .toImage(pixelRatio: 1);
                final bytes = await image.toByteData(
                  format: ui.ImageByteFormat.png,
                );
                File(
                  '${output.path}/${entry.key}-${width.toInt()}.png',
                ).writeAsBytesSync(bytes!.buffer.asUint8List());
                image.dispose();
              });
            }
            await tester.pumpWidget(const SizedBox());
            await tester.pumpAndSettle();
          }
        }
      }
    }
    tester.view.resetPhysicalSize();
    tester.view.resetDevicePixelRatio();
    File('../docs/test-output/app-v1.7-native-layout.json').writeAsStringSync(
      const JsonEncoder.withIndent('  ').convert({
        'renderer':
            'Flutter widget test with real local public data and uploaded images',
        'deviceValidation': false,
        'checks': report.length,
        'layouts': report,
      }),
    );
  }, skip: Platform.environment['APP_V17_CAPTURE'] != '1');
}
