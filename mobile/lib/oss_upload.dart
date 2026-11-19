part of 'main.dart';

extension CloudFileUpload on AppModel {
  Future<Json?> tryCloudUpload(
    String path, {
    String kind = 'image',
    String purpose = 'unknown',
    String? filename,
    void Function(int)? onProgress,
    http.Client? uploadClient,
    bool Function()? cancelled,
  }) async {
    if ((await clientCapabilities())['oss_direct_upload'] != true) return null;
    final file = File(path), size = await File(path).length();
    final modified = (await file.lastModified()).millisecondsSinceEpoch;
    final key =
        'oss.upload.$scope.${Uri.encodeComponent(path)}.$size.$modified.$kind.$purpose';
    final transport = uploadClient ?? http.Client();
    final source = baseUrl, owner = account['id'];
    String? id = prefs.getString(key);
    void check() {
      if (cancelled?.call() == true ||
          source != baseUrl ||
          owner != account['id']) {
        throw ApiFailure('upload_cancelled');
      }
    }

    Future<Json> call(
      String suffix, {
      Json? data,
      String method = 'POST',
    }) async {
      check();
      return asMap(
        await api('/merchant/files/uploads$suffix', method: method, data: data),
      );
    }

    Future<void> put(
      Json auth,
      int offset,
      int count,
      void Function(int) progress,
    ) async {
      check();
      final request = http.StreamedRequest('PUT', Uri.parse(auth['url']));
      asMap(auth['headers'] ?? {}).forEach((key, value) {
        if (![
          'authorization',
          'host',
          'content-length',
        ].contains(key.toLowerCase())) {
          request.headers[key] = value.toString();
        }
      });
      request.contentLength = count;
      var sent = 0;
      final responseFuture = transport
          .send(request)
          .timeout(const Duration(minutes: 5));
      final responseHandled = responseFuture.then((response) async {
        await response.stream.drain<void>();
        if (response.statusCode < 200 || response.statusCode >= 300) {
          throw ApiFailure(
            response.statusCode == 403
                ? 'oss_authorization_expired'
                : 'oss_upload_failed',
          );
        }
      });
      try {
        await request.sink.addStream(
          file.openRead(offset, offset + count).map((chunk) {
            check();
            sent += chunk.length;
            progress(sent);
            return chunk;
          }),
        );
        await request.sink.close();
        await responseHandled;
      } catch (_) {
        transport.close();
        await responseHandled.catchError((Object _) {});
        rethrow;
      }
    }

    try {
      Json? auth;
      Json? session;
      if (id != null) {
        session = asMap((await call('/$id', method: 'GET'))['session']);
        if (['cancelled', 'reclaimed'].contains(session['state']) ||
            DateTime.parse(session['expires_at']).isBefore(DateTime.now())) {
          id = null;
          await prefs.remove(key);
          await prefs.remove('$key.request');
        }
      }
      if (id == null) {
        final name = filename ?? path.replaceAll('\\', '/').split('/').last;
        final ext = name.toLowerCase().split('.').last;
        final mime = ext == 'pdf'
            ? 'application/pdf'
            : ext == 'mp4'
            ? 'video/mp4'
            : ext == 'mov'
            ? 'video/quicktime'
            : ext == 'png'
            ? 'image/png'
            : ext == 'webp'
            ? 'image/webp'
            : 'image/jpeg';
        final requestID =
            prefs.getString('$key.request') ?? newServiceCredential();
        await prefs.setString('$key.request', requestID);
        auth = await call(
          '',
          data: {
            'idempotency_key': requestID,
            'kind': kind,
            'purpose': purpose,
            'filename': name,
            'size': size,
            'mime_type': mime,
          },
        );
        session = asMap(auth['session']);
        id = session['id'];
        await prefs.setString(key, id!);
      }
      if (session!['state'] == 'failed') {
        await call('/$id/retry', data: {});
        session['state'] = 'validating';
      }
      if (session['state'] == 'uploading') {
        auth ??= await call('/$id/authorization', data: {});
        session = asMap(auth['session']);
        if (session['state'] == 'uploading' && auth['mode'] == 'single') {
          await put(
            auth,
            0,
            size,
            (n) => onProgress?.call((n / size * 100).floor()),
          );
        } else if (session['state'] == 'uploading') {
          final loaded = <int, int>{};
          var next = 1;
          final count = (auth['part_count'] as num).toInt();
          final uploaded = List<num>.from(
            auth['uploaded_parts'] ?? [],
          ).map((n) => n.toInt()).toSet();
          final partSize = (auth['part_size'] as num).toInt();
          for (final n in uploaded) {
            loaded[n] = min(partSize, size - (n - 1) * partSize);
          }
          Future<void> worker() async {
            while (next <= count) {
              check();
              final part = next++;
              if (uploaded.contains(part)) continue;
              final urls = await call(
                '/$id/parts',
                data: {
                  'parts': [part],
                },
              );
              final p = rows(urls['items']).first;
              await put(
                p,
                (p['offset'] as num).toInt(),
                (p['size'] as num).toInt(),
                (n) {
                  loaded[part] = n;
                  onProgress?.call(
                    (loaded.values.fold<int>(0, (a, b) => a + b) / size * 100)
                        .floor(),
                  );
                },
              );
            }
          }

          await Future.wait([worker(), worker()]);
        }
        await call('/$id/complete', data: {});
      }
      for (var n = 0; n < 450; n++) {
        check();
        session = asMap((await call('/$id', method: 'GET'))['session']);
        if (session['state'] == 'completed') {
          final media = (session['media_id'] as num).toInt();
          await prefs.remove(key);
          await prefs.remove('$key.request');
          onProgress?.call(100);
          return asMap(await api('/merchant/resource-media/$media'));
        }
        if (['failed', 'cancelled', 'reclaimed'].contains(session['state'])) {
          throw ApiFailure(session['error'] ?? 'oss_upload_failed');
        }
        await Future<void>.delayed(const Duration(seconds: 2));
      }
      throw ApiFailure('file_processing_timeout');
    } catch (_) {
      if (cancelled?.call() == true && id != null) {
        await api(
          '/merchant/files/uploads/$id/cancel',
          method: 'POST',
          data: {},
        ).catchError((Object _) => null);
        await prefs.remove(key);
        await prefs.remove('$key.request');
      }
      rethrow;
    } finally {
      if (uploadClient == null) transport.close();
    }
  }

  Future<Json> privateFileAccess(int id, {bool thumb = false}) async {
    if ((await clientCapabilities())['oss_file_access'] == true) {
      return asMap(
        await api('/merchant/files/$id/access${thumb ? '?thumb=1' : ''}'),
      );
    }
    return {
      'url': '$baseUrl/api/v1/media/$id${thumb ? '?thumb=1' : ''}',
      'direct': false,
    };
  }
}

class CloudNetworkImage extends StatefulWidget {
  final int mediaID;
  final bool thumb;
  final BoxFit? fit;
  final Alignment alignment;
  final double? width, height;
  final ImageLoadingBuilder? loadingBuilder;
  final ImageErrorWidgetBuilder? errorBuilder;
  const CloudNetworkImage({
    super.key,
    required this.mediaID,
    this.thumb = false,
    this.fit,
    this.alignment = Alignment.center,
    this.width,
    this.height,
    this.loadingBuilder,
    this.errorBuilder,
  });
  @override
  State<CloudNetworkImage> createState() => _CloudNetworkImageState();
}

class _CloudNetworkImageState extends State<CloudNetworkImage> {
  Future<Json>? access;
  String identity = '';
  int attempts = 0;
  bool renewing = false;
  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    reload();
  }

  @override
  void didUpdateWidget(covariant CloudNetworkImage old) {
    super.didUpdateWidget(old);
    reload();
  }

  void reload() {
    final m = ModelScope.of(context);
    final next = '${m.scope}:${widget.mediaID}:${widget.thumb}:$attempts';
    if (next == identity) return;
    identity = next;
    access = () async {
      if (m.token.isNotEmpty) {
        try {
          return await m.privateFileAccess(widget.mediaID, thumb: widget.thumb);
        } on ApiFailure catch (e) {
          if (e.code != 'not_found') rethrow;
        }
      }
      return {
        'url':
            '${m.baseUrl}/api/v1/media/${widget.mediaID}${widget.thumb ? '?thumb=1' : ''}',
        'direct': true,
      };
    }();
  }

  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    return FutureBuilder<Json>(
      future: access,
      builder: (context, snap) {
        if (snap.hasError) {
          return widget.errorBuilder?.call(
                context,
                ApiFailure('network_error'),
                null,
              ) ??
              const Icon(Icons.broken_image_outlined);
        }
        if (!snap.hasData) {
          return const Center(child: CircularProgressIndicator());
        }
        final a = snap.data!;
        return Image.network(
          a['url'],
          headers: a['direct'] == true
              ? {}
              : {'Authorization': 'Bearer ${m.token}'},
          fit: widget.fit,
          alignment: widget.alignment,
          width: widget.width,
          height: widget.height,
          loadingBuilder: widget.loadingBuilder,
          errorBuilder: (context, error, stack) {
            if (!renewing && attempts == 0 && a['direct'] == true) {
              renewing = true;
              WidgetsBinding.instance.addPostFrameCallback((_) {
                if (mounted) {
                  setState(() {
                    attempts++;
                    identity = '';
                    reload();
                  });
                }
              });
              return const Center(child: CircularProgressIndicator());
            }
            return widget.errorBuilder?.call(
                  context,
                  ApiFailure('network_error'),
                  stack,
                ) ??
                const Icon(Icons.broken_image_outlined);
          },
        );
      },
    );
  }
}

class CloudPdfViewer extends StatefulWidget {
  final int mediaID;
  final bool private;
  final PdfViewerParams params;
  final PdfViewerController controller;
  const CloudPdfViewer({
    super.key,
    required this.mediaID,
    required this.private,
    required this.params,
    required this.controller,
  });
  @override
  State<CloudPdfViewer> createState() => _CloudPdfViewerState();
}

class _CloudPdfViewerState extends State<CloudPdfViewer> {
  Future<Json>? access;
  String identity = '';
  int retry = 0;
  int lastPage = 1;
  void load() {
    final m = ModelScope.of(context);
    final next = '${m.scope}:${widget.mediaID}:${widget.private}:$retry';
    if (next == identity) return;
    identity = next;
    access = widget.private
        ? m.privateFileAccess(widget.mediaID)
        : Future.value({
            'url': '${m.baseUrl}/api/v1/media/${widget.mediaID}',
            'direct': true,
          });
  }

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    load();
  }

  @override
  void didUpdateWidget(covariant CloudPdfViewer old) {
    super.didUpdateWidget(old);
    load();
  }

  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    return FutureBuilder<Json>(
      future: access,
      builder: (context, snap) {
        if (snap.hasError) {
          return TextButton(
            onPressed: () => setState(() {
              retry++;
              load();
            }),
            child: Text(m.t('retry')),
          );
        }
        if (!snap.hasData) {
          return const Center(child: CircularProgressIndicator());
        }
        final a = snap.data!;
        return PdfViewer.uri(
          Uri.parse(a['url']),
          key: ValueKey(identity),
          headers: a['direct'] == true
              ? {}
              : {'Authorization': 'Bearer ${m.token}'},
          controller: widget.controller,
          initialPageNumber: lastPage,
          params: PdfViewerParams(
            onViewerReady: widget.params.onViewerReady,
            onPageChanged: (page) {
              if (page != null) lastPage = page;
              widget.params.onPageChanged?.call(page);
            },
            errorBannerBuilder: (context, error, stack, document) => TextButton(
              onPressed: () => setState(() {
                retry++;
                load();
              }),
              child: Text(m.t('retry')),
            ),
          ),
          preferRangeAccess: true,
        );
      },
    );
  }
}
