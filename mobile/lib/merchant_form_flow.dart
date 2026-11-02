part of 'main.dart';

/// Only owned product fields are copied. Reviews and resources belong to the
/// source content and must never become part of a new product.
Json cloneProductDraft(Json source) => {
  for (final key in [
    'name',
    'name_en',
    'product_category_id',
    'category_id',
    'brand',
    'model',
    'applicable_brands',
    'applicable_models',
    'keywords',
    'description',
    'description_en',
    'parameters',
    'media_ids',
  ])
    if (source.containsKey(key)) key: jsonDecode(jsonEncode(source[key])),
};

extension _MerchantFormFlow on _EditorScreenState {
  bool get existingCompany => widget.kind == 'vendor' && widget.item != null;
  bool get locked => busy || current?['status'] == 'pending';
  bool get cannotSave => locked || (creationUncertain && !safeCreationRetry);
  String get companySection => widget.initialSection ?? '';

  Future<void> productSubmitted() async {
    final m = ModelScope.of(context);
    final next = await showDialog<bool>(
      context: context,
      builder: (c) => AlertDialog(
        title: Text(syncText(c, '产品已提交审核', 'Product submitted')),
        content: Text(
          syncText(
            c,
            '审核通过后展示，您可以继续添加下一个产品。',
            'It will appear after review. You can add another product now.',
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(c, false),
            child: Text(syncText(c, '返回产品列表', 'Product list')),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(c, true),
            child: Text(syncText(c, '继续添加产品', 'Add another product')),
          ),
        ],
      ),
    );
    if (!mounted) return;
    if (next == true) {
      updateForm(() {
        dirty = false;
        leaving = true;
      });
      Navigator.pushReplacement(
        context,
        MaterialPageRoute(builder: (_) => const EditorScreen(kind: 'product')),
      );
    } else {
      dirty = false;
      leaving = true;
      Navigator.pop(context, true);
    }
    m.merchant.refresh();
  }

  Future<void> galleryMenu(Json entry, int index, int length) async {
    final action = await showModalBottomSheet<String>(
      context: context,
      builder: (c) => SafeArea(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            for (final (value, zh, en, icon) in [
              if (index > 0)
                ('cover', '设为封面', 'Set cover', Icons.photo_outlined),
              if (index > 0)
                ('earlier', '前移', 'Move earlier', Icons.arrow_back),
              if (index < length - 1)
                ('later', '后移', 'Move later', Icons.arrow_forward),
              ('replace', '替换', 'Replace', Icons.swap_horiz),
              ('delete', '删除', 'Delete', Icons.delete_outline),
            ])
              ListTile(
                leading: Icon(icon),
                title: Text(syncText(c, zh, en)),
                onTap: () => Navigator.pop(c, value),
              ),
          ],
        ),
      ),
    );
    if (!mounted || locked) return;
    switch (action) {
      case 'cover':
        updateForm(() {
          galleryOrder.remove(entry['key']);
          galleryOrder.insert(0, entry['key']);
          galleryChanged();
        });
      case 'earlier':
        moveImage(entry, -1);
      case 'later':
        moveImage(entry, 1);
      case 'replace':
        await replaceImage(entry);
      case 'delete':
        removeImage(entry);
    }
  }

  Widget compactGallery() {
    final m = ModelScope.of(context), entries = galleryEntries();
    return Column(
      key: sectionKeys['gallery'],
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Text(
          syncText(
            context,
            widget.kind == 'product' ? '产品图片' : '企业图片',
            widget.kind == 'product' ? 'Product photos' : 'Company photos',
          ),
          style: Theme.of(context).textTheme.titleMedium,
        ),
        Text(
          syncText(
            context,
            '${entries.length}/12张 · 首张为封面，点击图片管理',
            '${entries.length}/12 photos · First is the cover. Tap to manage.',
          ),
        ),
        if (issues['media_ids'] != null)
          Text(
            syncText(
              context,
              '提交前请添加至少一张产品图片',
              'Add at least one photo before submitting',
            ),
            style: const TextStyle(color: Colors.red),
          ),
        space(8),
        GridView.builder(
          shrinkWrap: true,
          physics: const NeverScrollableScrollPhysics(),
          gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
            crossAxisCount: 3,
            crossAxisSpacing: 8,
            mainAxisSpacing: 8,
          ),
          itemCount: entries.length,
          itemBuilder: (c, i) {
            final entry = entries[i];
            return Semantics(
              button: true,
              label: '${i == 0 ? syncText(c, '封面', 'Cover') : i + 1}',
              child: InkWell(
                onTap: locked
                    ? null
                    : () => galleryMenu(entry, i, entries.length),
                child: ClipRRect(
                  borderRadius: BorderRadius.circular(10),
                  child: Stack(
                    fit: StackFit.expand,
                    children: [
                      entry['id'] != null
                          ? Photo(
                              id: entry['id'],
                              fit: BoxFit.contain,
                              height: 120,
                            )
                          : Image.file(
                              File(entry['path']),
                              fit: BoxFit.contain,
                              errorBuilder: (_, _, _) =>
                                  const Icon(Icons.broken_image_outlined),
                            ),
                      Positioned(
                        left: 0,
                        right: 0,
                        bottom: 0,
                        child: Container(
                          color: Colors.black54,
                          padding: const EdgeInsets.all(4),
                          child: Text(
                            '${i == 0 ? syncText(c, '封面', 'Cover') : i + 1}${entry['id'] == null ? (entry['error'] != null ? syncText(c, ' · 上传失败', ' · Retry') : syncText(c, ' · 待上传', ' · Pending')) : ''}',
                            style: const TextStyle(
                              color: Colors.white,
                              fontSize: 11,
                            ),
                            maxLines: 2,
                          ),
                        ),
                      ),
                      if (!locked)
                        const Positioned(
                          top: 4,
                          right: 4,
                          child: Icon(
                            Icons.more_horiz,
                            color: Color(0xff1769c2),
                          ),
                        ),
                    ],
                  ),
                ),
              ),
            );
          },
        ),
        if (busy)
          LinearProgressIndicator(
            value: brandProgress > 0 ? brandProgress / 100 : null,
          ),
        Wrap(
          spacing: 8,
          children: [
            OutlinedButton.icon(
              onPressed: !locked && entries.length < 12
                  ? () => pick(ImageSource.camera)
                  : null,
              icon: const Icon(Icons.camera_alt_outlined),
              label: Text(m.t('camera')),
            ),
            OutlinedButton.icon(
              onPressed: !locked && entries.length < 12
                  ? () => pick(ImageSource.gallery)
                  : null,
              icon: const Icon(Icons.photo_library_outlined),
              label: Text(m.t('album')),
            ),
            if (photos.any((p) => p['id'] == null && p['slot'] == null))
              TextButton(
                onPressed: locked ? null : retryGallery,
                child: Text(m.t('retry')),
              ),
          ],
        ),
      ],
    );
  }

  Widget profileField(String key, String zh, String en, {bool year = false}) {
    final profile = asMap(data['company_profile'] ?? {}),
        path = 'company_profile.$key';
    final controller = nestedFields.putIfAbsent(
      path,
      () => TextEditingController(text: (profile[key] ?? '').toString()),
    );
    return Padding(
      key: fieldKeys.putIfAbsent(path, () => GlobalKey()),
      padding: const EdgeInsets.only(bottom: 16),
      child: NativeFieldLabel(
        label: syncText(context, zh, en),
        child: TextField(
          controller: controller,
          enabled: !locked,
          keyboardType: year ? TextInputType.number : TextInputType.text,
          decoration: guidedDecoration(
            ModelScope.of(context),
            path,
            controller.text,
            zh,
            issue: issues[path],
          ),
          onChanged: (v) {
            updateForm(
              () => data['company_profile'] = {
                ...asMap(data['company_profile'] ?? {}),
                key: v,
              },
            );
            scheduleSave();
          },
        ),
      ),
    );
  }

  Widget companyImages() {
    final profile = asMap(data['company_profile'] ?? {});
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        for (final slot in ['logo_media_id', 'cover_media_id'])
          NativeBrandImageEditor(
            slot: slot,
            value: profile[slot] ?? 0,
            ids: imageIDs(data),
            pending: photos
                .where((p) => p['slot'] == slot && p['id'] == null)
                .firstOrNull,
            enabled: !locked,
            progress: brandProgress,
            onChanged: (id) {
              updateForm(
                () => data['company_profile'] = {...profile, slot: id},
              );
              scheduleSave();
            },
            onPick: (file) => pickBrand(slot, file),
            onRetry: () => unawaited(retryBrand()),
            onDiscard: () {
              updateForm(() => photos.removeWhere((p) => p['slot'] == slot));
              scheduleSave();
            },
          ),
        gallery(),
      ],
    );
  }

  Widget companyContacts() {
    final profile = asMap(data['company_profile'] ?? {}),
        contacts = rows(profile['contacts']);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Text(
          syncText(
            context,
            '联系方式修改后需审核，审核通过后更新公开资料。',
            'Contact changes require review before publication.',
          ),
        ),
        space(),
        field(context, 'phone', 'phone'),
        field(context, 'wechat', 'wechat'),
        field(context, 'address', 'address'),
        profileField('contact_name', '联系人', 'Contact person'),
        Text(ModelScope.of(context).t('regionalContacts')),
        for (var i = 0; i < contacts.length; i++)
          Card(
            child: Padding(
              padding: const EdgeInsets.all(12),
              child: Column(
                children: [
                  for (final (key, zh, en) in [
                    ('area', '服务地区', 'Service area'),
                    ('name', '联系人', 'Contact person'),
                    ('phone', '电话', 'Phone'),
                    ('wechat', '微信', 'WeChat'),
                  ])
                    contactField(contacts, i, key, zh, en),
                  TextButton(
                    onPressed: locked
                        ? null
                        : () {
                            updateForm(() {
                              contacts.removeAt(i);
                              data['company_profile'] = {
                                ...profile,
                                'contacts': contacts,
                              };
                              for (final entry in nestedFields.entries.where(
                                (e) => e.key.startsWith(
                                  'company_profile.contacts.',
                                ),
                              )) {
                                final parts = entry.key.split('.');
                                final index = int.parse(parts[2]);
                                entry.value.text = index < contacts.length
                                    ? (contacts[index][parts[3]] ?? '')
                                          .toString()
                                    : '';
                              }
                            });
                            scheduleSave();
                          },
                    child: Text(ModelScope.of(context).t('remove')),
                  ),
                ],
              ),
            ),
          ),
        OutlinedButton(
          onPressed: locked || contacts.length >= 20
              ? null
              : () {
                  updateForm(
                    () => data['company_profile'] = {
                      ...profile,
                      'contacts': [
                        ...contacts,
                        {'area': '', 'name': '', 'phone': '', 'wechat': ''},
                      ],
                    },
                  );
                  scheduleSave();
                },
          child: Text(ModelScope.of(context).t('addContact')),
        ),
      ],
    );
  }

  Widget contactField(
    List<Json> contacts,
    int i,
    String key,
    String zh,
    String en,
  ) {
    final path = 'company_profile.contacts.$i.$key',
        controller = nestedFields.putIfAbsent(
          'company_profile.contacts.$i.$key',
          () =>
              TextEditingController(text: (contacts[i][key] ?? '').toString()),
        );
    return Padding(
      padding: const EdgeInsets.only(bottom: 12),
      child: NativeFieldLabel(
        label: syncText(context, zh, en),
        child: TextField(
          controller: controller,
          enabled: !locked,
          keyboardType: key == 'phone'
              ? TextInputType.phone
              : TextInputType.text,
          decoration: guidedDecoration(
            ModelScope.of(context),
            path,
            controller.text,
            zh,
            issue: issues[path],
          ),
          onChanged: (v) {
            updateForm(() {
              contacts[i] = {...contacts[i], key: v};
              data['company_profile'] = {
                ...asMap(data['company_profile'] ?? {}),
                'contacts': contacts,
              };
            });
            scheduleSave();
          },
        ),
      ),
    );
  }

  Widget companyMore() => Column(
    crossAxisAlignment: CrossAxisAlignment.stretch,
    children: [
      group('company-size', '规模与成立年份（选填）', 'Company size and year (optional)', [
        profileField('established', '成立年份', 'Year established', year: true),
        profileField('staff', '员工人数', 'Staff'),
        profileField('factory_area', '厂房面积', 'Factory area'),
        profileField('equipment', '主要设备', 'Equipment'),
        profileField('capacity', '生产能力', 'Capacity'),
      ]),
      group('english', '英文资料（选填）', 'English details (optional)', [
        field(context, 'name_en', 'nameEN'),
        field(context, 'description_en', 'englishDescription', multiline: true),
        for (final (key, zh, en) in [
          ('short_name_en', '英文简称', 'Short name'),
          ('summary_en', '英文概述', 'Summary'),
          ('equipment_en', '英文设备介绍', 'Equipment'),
          ('capacity_en', '英文产能介绍', 'Capacity'),
          ('contact_name_en', '英文联系人', 'Contact person'),
        ])
          profileField(key, zh, en),
      ]),
      group(
        'credentials',
        '资质与图片说明（选填）',
        'Credentials and photo captions (optional)',
        [
          PortalMediaEditor(
            data: data,
            issues: issues,
            enabled: !locked,
            photos: photos,
            onPick: pickBrand,
            onRetry: retryBrand,
            onDiscard: (slot) {
              updateForm(() => photos.removeWhere((p) => p['slot'] == slot));
              scheduleSave();
            },
            onChanged: (v) {
              updateForm(() => data = v);
              scheduleSave();
            },
          ),
        ],
      ),
    ],
  );

  Widget companySectionBody() {
    switch (companySection) {
      case 'brand':
      case 'gallery':
        return companyImages();
      case 'video':
        return NativeResourceList(kind: 'video', management: true);
      case 'contact':
        return companyContacts();
      case 'more':
      case 'credentials':
      case 'profile':
        return companyMore();
      case 'review':
        return previewBody();
      default:
        return Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            companyBasics(),
            profileField('short_name', '企业简称（选填）', 'Short name (optional)'),
            profileField('summary', '企业概述（选填）', 'Summary (optional)'),
            profileField('website', '企业网站（选填）', 'Website (optional)'),
          ],
        );
    }
  }

  Widget companyHome() => ListView(
    padding: const EdgeInsets.all(20),
    children: [
      MerchantCompanyCard(item: current!),
      space(12),
      for (final (section, zh, en, icon) in [
        ('basic', '企业资料', 'Company profile', Icons.business_outlined),
        ('brand', '封面与图片', 'Cover and photos', Icons.photo_library_outlined),
        ('video', '企业视频', 'Company videos', Icons.video_library_outlined),
        ('products', '产品管理', 'Products', Icons.inventory_2_outlined),
        ('contact', '联系方式', 'Contact details', Icons.contact_phone_outlined),
        ('more', '更多资料', 'More details', Icons.more_horiz),
      ])
        Card(
          child: ListTile(
            leading: Icon(icon),
            title: Text(syncText(context, zh, en)),
            trailing: const Icon(Icons.chevron_right),
            onTap: () => section == 'products'
                ? push(context, const MerchantProductsScreen())
                : editMerchant(context, current, 'vendor', section: section),
          ),
        ),
      OutlinedButton.icon(
        onPressed: () => push(
          context,
          PreviewScreen(data: data, kind: 'vendor', photos: photos),
        ),
        icon: const Icon(Icons.visibility_outlined),
        label: Text(ModelScope.of(context).t('preview')),
      ),
    ],
  );

  Widget stepHeader(bool product) => Row(
    children: [
      for (var i = 0; i < 3; i++)
        Expanded(
          child: TextButton(
            style: TextButton.styleFrom(
              padding: const EdgeInsets.symmetric(horizontal: 2),
              backgroundColor: productStep == i
                  ? const Color(0xffe6f0ff)
                  : null,
            ),
            onPressed: busy
                ? null
                : () {
                    updateForm(() => productStep = i);
                    scheduleSave();
                  },
            child: Text(
              '${i + 1} ${syncText(context, (product ? ['添加产品', '完善详情', '预览发布'] : ['企业资料', '展示资料', '预览提交'])[i], (product ? ['Add product', 'Details', 'Preview'] : ['Profile', 'Presentation', 'Preview'])[i])}',
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
            ),
          ),
        ),
    ],
  );

  Widget formActions(bool product) {
    final m = ModelScope.of(context);
    if (existingCompany &&
        (companySection.isEmpty || companySection == 'video')) {
      return const SizedBox.shrink();
    }
    return Material(
      elevation: 8,
      color: Theme.of(context).colorScheme.surface,
      child: SafeArea(
        top: false,
        child: Padding(
          padding: const EdgeInsets.fromLTRB(12, 8, 12, 8),
          child: Row(
            children: [
              if (!existingCompany && productStep > 0)
                IconButton(
                  tooltip: m.t('previous'),
                  onPressed: busy
                      ? null
                      : () {
                          updateForm(() => productStep--);
                          scheduleSave();
                        },
                  icon: const Icon(Icons.arrow_back),
                ),
              Expanded(
                child: OutlinedButton(
                  onPressed: cannotSave ? null : () => save(),
                  child: Text(m.t(busy ? 'loading' : 'saveCloud'), maxLines: 1),
                ),
              ),
              const SizedBox(width: 8),
              Expanded(
                child: FilledButton(
                  onPressed: cannotSave
                      ? null
                      : (!existingCompany && productStep < 2
                            ? () {
                                FocusScope.of(context).unfocus();
                                updateForm(() => productStep++);
                                scheduleSave();
                              }
                            : () => save(submit: true)),
                  child: Text(
                    !existingCompany && productStep < 2
                        ? m.t('next')
                        : syncText(context, '提交审核', 'Submit for review'),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                  ),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget buildFormFlow() {
    final m = ModelScope.of(context),
        product = widget.kind == 'product',
        home = existingCompany && companySection.isEmpty;
    return PopScope(
      canPop: !dirty || leaving,
      onPopInvokedWithResult: (didPop, _) async {
        if (didPop) return;
        await persist();
        if (!mounted) return;
        updateForm(() => leaving = true);
        WidgetsBinding.instance.addPostFrameCallback((_) {
          if (mounted) Navigator.pop(context);
        });
      },
      child: Theme(
        data: merchantTheme(context),
        child: Scaffold(
          appBar: appBar(
            context,
            product
                ? syncText(
                    context,
                    current == null ? '添加产品' : '编辑产品',
                    current == null ? 'Add product' : 'Edit product',
                  )
                : syncText(
                    context,
                    home ? '企业管理' : '企业资料',
                    home ? 'Company management' : 'Company profile',
                  ),
          ),
          bottomNavigationBar: formActions(product),
          body: SafeArea(
            child: home
                ? companyHome()
                : SingleChildScrollView(
                    padding: const EdgeInsets.all(20),
                    child: Center(
                      child: ConstrainedBox(
                        constraints: const BoxConstraints(maxWidth: 840),
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.stretch,
                          children: [
                            if (!existingCompany) stepHeader(product),
                            if (current?['status'] == 'pending')
                              UpgradeNotice(
                                syncText(
                                  context,
                                  '资料正在审核，审核期间可查看。公开版本继续展示。',
                                  'This version is under review. The published version remains visible.',
                                ),
                              ),
                            if (issues.isNotEmpty)
                              UpgradeNotice(
                                syncText(
                                  context,
                                  '请修改提示项目，填写内容已保留',
                                  'Correct the highlighted fields. Your input is retained.',
                                ),
                              ),
                            if (creationUncertain)
                              UpgradeNotice(
                                idempotencySupported
                                    ? syncText(
                                        context,
                                        '上次保存尚未确认，点击保存可安全重试。',
                                        'The last save was not confirmed. Save again to safely retry.',
                                      )
                                    : syncText(
                                        context,
                                        '上次保存尚未确认，请返回产品列表核对。',
                                        'The last save was not confirmed. Check your product list.',
                                      ),
                              ),
                            if (existingCompany) ...[
                              companySectionBody(),
                              if (companySection != 'video')
                                const NativePublicationAgreements(
                                  hideAccepted: true,
                                ),
                            ] else if (product)
                              Column(
                                children: [
                                  for (final (i, body) in [
                                    productBasics(),
                                    productDetails(),
                                    previewBody(),
                                  ].indexed)
                                    Visibility(
                                      visible: productStep == i,
                                      maintainState: true,
                                      child: body,
                                    ),
                                ],
                              )
                            else ...[
                              if (productStep == 0) companyBasics(),
                              if (productStep == 1)
                                Column(children: [companyExtras(), gallery()]),
                              if (productStep == 2)
                                Column(
                                  children: [
                                    previewBody(),
                                    const NativePublicationAgreements(
                                      hideAccepted: true,
                                    ),
                                  ],
                                ),
                            ],
                            space(16),
                            Text(
                              m.t('localSavedHint'),
                              style: const TextStyle(fontSize: 12),
                            ),
                            space(24),
                          ],
                        ),
                      ),
                    ),
                  ),
          ),
        ),
      ),
    );
  }
}
