part of 'main.dart';

class LegacyPhoneBindingScreen extends StatefulWidget {
  final bool visitor, register, binding, companyAccount;
  final String initialName;
  final VoidCallback? onRegistered;
  const LegacyPhoneBindingScreen({
    super.key,
    this.visitor = false,
    this.register = false,
    this.binding = false,
    this.companyAccount = true,
    this.initialName = '',
    this.onRegistered,
  });
  @override
  State<LegacyPhoneBindingScreen> createState() => _LoginScreenState();
}

class _LoginScreenState extends State<LegacyPhoneBindingScreen> {
  final controls = {
    for (final k in ['phone', 'code', 'password', 'confirmation', 'login_name'])
      k: TextEditingController(),
  };
  final focuses = {
    for (final k in ['phone', 'code', 'password', 'confirmation', 'login_name'])
      k: FocusNode(),
  };
  bool smsMode = false;
  bool registerMode = false,
      legacy = false,
      setup = false,
      busy = false,
      visible = false,
      confirmationVisible = false,
      unconfirmed = false;
  List<Json> acceptedAgreements = [];
  int agreementGeneration = 0;
  Map<String, String> errors = {};
  String error = '', dev = '';
  String smsRequestKey = '', smsRequestPhone = '', smsRequestPurpose = '';
  String get smsPurpose => widget.binding
      ? 'bind'
      : smsMode
      ? 'login'
      : setup
      ? 'setup_password'
      : 'register';
  int cooldown = 0;
  Timer? timer;
  bool get verifying =>
      !legacy && (widget.binding || registerMode || setup || smsMode);
  String value(String k) => controls[k]!.text;
  String text(String z, String e) => syncText(context, z, e);
  @override
  void initState() {
    super.initState();
    registerMode = widget.register;
    legacy = widget.companyAccount && !widget.visitor && !widget.binding;
    controls['login_name']!.text = widget.initialName;
  }

  @override
  void dispose() {
    for (final c in controls.values) {
      c.dispose();
    }
    for (final f in focuses.values) {
      f.dispose();
    }
    timer?.cancel();
    super.dispose();
  }

  void mode({
    bool registration = false,
    bool old = false,
    bool setting = false,
  }) {
    if (busy) return;
    setState(() {
      smsMode = false;
      registerMode = registration;
      legacy = old;
      setup = setting;
      for (final k in ['password', 'confirmation', 'code']) {
        controls[k]!.clear();
      }
      visible = confirmationVisible = unconfirmed = false;
      errors = {};
      error = dev = '';
    });
  }

  void focusErrors() {
    final key = [
      'phone',
      'login_name',
      'code',
      'password',
      'confirmation',
    ].firstWhere(errors.containsKey, orElse: () => 'phone');
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      final n = focuses[key]!;
      n.requestFocus();
      if (n.context != null) {
        unawaited(
          Scrollable.ensureVisible(
            n.context!,
            alignment: .25,
            duration: const Duration(milliseconds: 200),
          ),
        );
      }
    });
  }

  Future<Json?> smsProof(AppModel m) async {
    Json image = asMap(
      await m.api(
        '/auth/phone/challenge',
        method: 'POST',
        data: {'phone': value('phone').trim(), 'purpose': smsPurpose},
      ),
    );
    if (!mounted) return null;
    String answer = "";
    bool loading = false;
    String dialogError = '';
    return await showDialog<Json>(
      context: context,
      builder: (dialogContext) => StatefulBuilder(
        builder: (context, setDialogState) {
          final uri = (image['image'] ?? '').toString();
          return AlertDialog(
            title: Text(text('图形验证码', 'Image verification')),
            content: SingleChildScrollView(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  if (uri.startsWith('data:image/png;base64,'))
                    Image.memory(
                      base64Decode(uri.split(',').last),
                      width: 156,
                      height: 52,
                    ),
                  TextField(
                    key: ValueKey('sms-image-answer-${image['challenge_id']}'),
                    onChanged: (value) => answer = value,
                    keyboardType: TextInputType.number,
                    inputFormatters: [
                      FilteringTextInputFormatter.digitsOnly,
                      LengthLimitingTextInputFormatter(5),
                    ],
                    decoration: InputDecoration(
                      labelText: text('输入图片中的五位数字', 'Enter the five digits'),
                    ),
                  ),
                  if (dialogError.isNotEmpty) Text(dialogError),
                  TextButton(
                    onPressed: loading
                        ? null
                        : () async {
                            setDialogState(() => loading = true);
                            try {
                              final next = asMap(
                                await m.api(
                                  '/auth/phone/challenge',
                                  method: 'POST',
                                  data: {
                                    'phone': value('phone').trim(),
                                    'purpose': smsPurpose,
                                  },
                                ),
                              );
                              if (context.mounted) {
                                setDialogState(() {
                                  image = next;
                                  answer = '';
                                  dialogError = '';
                                });
                              }
                            } catch (_) {
                              if (context.mounted) {
                                setDialogState(
                                  () => dialogError = text(
                                    '加载失败，请重试',
                                    'Loading failed. Retry.',
                                  ),
                                );
                              }
                            } finally {
                              if (context.mounted) {
                                setDialogState(() => loading = false);
                              }
                            }
                          },
                    child: Text(text('换一张', 'Refresh')),
                  ),
                ],
              ),
            ),
            actions: [
              TextButton(
                onPressed: () => Navigator.pop(dialogContext),
                child: Text(m.t('cancel')),
              ),
              FilledButton(
                onPressed: loading
                    ? null
                    : () {
                        if (!RegExp(r'^\d{5}$').hasMatch(answer)) {
                          setDialogState(
                            () => dialogError = text(
                              '请填写五位数字',
                              'Enter five digits',
                            ),
                          );
                          return;
                        }
                        Navigator.pop(dialogContext, {
                          'challenge_id': image['challenge_id'],
                          'challenge_answer': answer,
                        });
                      },
                child: Text(text('验证并发送', 'Verify and send')),
              ),
            ],
          );
        },
      ),
    );
  }

  Future<void> submit({bool send = false}) async {
    if (busy || send && cooldown > 0 || !send && unconfirmed) return;
    final m = ModelScope.of(context),
        source = m.baseUrl,
        invalid = <String, String>{};
    if (legacy) {
      if (value('login_name').trim().isEmpty ||
          value('login_name').trim().runes.length > 100) {
        invalid['login_name'] = text(
          '账号不能为空，最多100字',
          'Required, up to 100 characters',
        );
      }
    } else if (!RegExp(r'^1[3-9][0-9]{9}$').hasMatch(value('phone').trim())) {
      invalid['phone'] = text(
        '请填写中国大陆11位手机号',
        'Enter an 11-digit mainland China mobile number',
      );
    }
    if (!send) {
      if (verifying && !RegExp(r'^\d{6}$').hasMatch(value('code').trim())) {
        invalid['code'] = text('请填写6位验证码', 'Enter the 6-digit code');
      }
      if (!widget.binding && !smsMode) {
        if ((registerMode || setup) && !validNewPassword(value('password'))) {
          invalid['password'] = text(
            '请输入8–20位密码',
            'Enter an 8–20 character password',
          );
        } else if (value('password').isEmpty) {
          invalid['password'] = text('请填写密码', 'Enter your password');
        }
        if ((registerMode || setup) &&
            value('password') != value('confirmation')) {
          invalid['confirmation'] = text('两次密码不一致', 'Passwords do not match');
        }
      }
    }
    setState(() {
      errors = invalid;
      error = '';
    });
    if (invalid.isNotEmpty) {
      focusErrors();
      return;
    }
    if (registerMode && !send && acceptedAgreements.length != 2) {
      setState(() => error = m.t('agreement_required'));
      return;
    }
    setState(() => busy = true);
    bool registrationRequestStarted = false;
    try {
      if (!legacy) {
        var capabilities = await m.clientCapabilities();
        final capability = send && registerMode
            ? 'phone_sms_register'
            : smsMode
            ? 'phone_sms_auth'
            : 'phone_password_auth';
        if (capabilities[capability] != true) {
          capabilities = await m.clientCapabilities(refresh: true);
        }
        if (capabilities[capability] != true) {
          throw ApiFailure('phoneAuthUpgradeRequired');
        }
      }
      if (!mounted || m.baseUrl != source) return;
      final path = legacy
          ? (registerMode
                ? (widget.visitor
                      ? '/auth/visitor/register'
                      : '/auth/merchant/register')
                : (widget.visitor ? '/auth/visitor' : '/auth/merchant'))
          : widget.binding
          ? (send ? '/auth/me/phone/code' : '/auth/me/phone')
          : send
          ? '/auth/phone/code'
          : smsMode
          ? '/auth/phone/login'
          : setup
          ? '/auth/phone/password/setup'
          : registerMode
          ? '/auth/phone/register'
          : '/auth/phone/password';
      Json proof = {};
      if (send) {
        if (smsRequestPhone != value('phone').trim() ||
            smsRequestPurpose != smsPurpose) {
          smsRequestKey = '';
        }
        smsRequestPhone = value('phone').trim();
        smsRequestPurpose = smsPurpose;
        if (smsRequestKey.isEmpty) smsRequestKey = newServiceCredential();
        if ((await m.clientCapabilities())['sms_challenge'] == true) {
          final verified = await smsProof(m);
          if (verified == null || !mounted || source != m.baseUrl) return;
          proof = verified;
        }
      }
      registrationRequestStarted = registerMode && !send;
      final result = asMap(
        await m.api(
          path,
          method: widget.binding && !send ? 'PUT' : 'POST',
          data: {
            if (send) ...proof,
            if (send) 'request_id': smsRequestKey,
            if (registerMode && !send) 'agreements': acceptedAgreements,
            if (registerMode && !send)
              'source': Platform.isIOS ? 'ios' : 'android',
            if (legacy)
              (registerMode ? 'login_name' : 'username'): value(
                'login_name',
              ).trim()
            else
              'phone': value('phone').trim(),
            if (send && !widget.binding)
              'purpose': smsMode
                  ? 'login'
                  : setup
                  ? 'setup_password'
                  : 'register',
            if (!send && verifying) 'code': value('code').trim(),
            if (!send && !widget.binding && !smsMode)
              'password': value('password'),
            if (!send && registerMode) 'role': 'visitor',
          },
        ),
      );
      if (!mounted || source != m.baseUrl) return;
      if (send) {
        smsRequestKey = '';
        setState(() {
          dev = (result['development_code'] ?? '').toString();
          cooldown = (result['retry_after'] as num?)?.toInt() ?? 60;
        });
        timer?.cancel();
        timer = Timer.periodic(const Duration(seconds: 1), (t) {
          if (!mounted) {
            t.cancel();
            return;
          }
          setState(() => cooldown--);
          if (cooldown <= 0) t.cancel();
        });
      } else {
        await m.signIn(result);
        controls['password']!.clear();
        controls['confirmation']!.clear();
        if (!mounted || source != m.baseUrl) return;
        if (widget.binding) {
          Navigator.pop(context, true);
          return;
        }
        if (legacy && m.account['requires_phone_binding'] == true) {
          final capable =
              (await m.clientCapabilities())['phone_sms_bind'] == true;
          if (!mounted) return;
          if (capable) {
            await Navigator.push<bool>(
              context,
              MaterialPageRoute(
                builder: (_) => const LoginScreen(binding: true),
              ),
            );
          }
        }
        if (!mounted || source != m.baseUrl) return;
        if (!widget.visitor && m.account['role'] == 'visitor') {
          if (!await confirmMerchantUpgrade(context)) return;
          if (!mounted) return;
          widget.onRegistered?.call();
        } else if (registerMode && !widget.visitor) {
          widget.onRegistered?.call();
        }
        if (mounted) Navigator.pop(context, true);
      }
    } catch (e) {
      if (!mounted || source != m.baseUrl) return;
      final f = e is ApiFailure ? e : ApiFailure('network_error');
      if (send && f.code != 'network_error') smsRequestKey = '';
      setState(() {
        if (f.code == 'agreement_required') {
          acceptedAgreements = [];
          agreementGeneration++;
        }
        if (f.fields.isNotEmpty) {
          errors = {for (final x in f.fields) x['field']: fieldIssueText(m, x)};
        } else {
          error = m.t(f.code);
        }
        if ([
          'phone_exists',
          'phone_already_bound',
          'password_setup_unavailable',
        ].contains(f.code)) {
          error = m.t(f.code);
        }
        if (f.code == 'invalid_code') {
          errors['code'] = text(
            '验证码错误、已过期或已使用，请重新获取',
            'The code is incorrect, expired or already used. Request a new code.',
          );
        }
        if (registrationRequestStarted && f.code == 'network_error') {
          unconfirmed = true;
          error = text(
            '无法确认账号是否已注册。请用刚填写的账号和密码登录继续。',
            'Registration could not be confirmed. Sign in with the account and password you just entered.',
          );
        }
      });
      if (errors.isNotEmpty) focusErrors();
    } finally {
      if (mounted) setState(() => busy = false);
    }
  }

  Widget input(
    String key,
    String label, {
    bool secret = false,
    bool confirm = false,
  }) => NativeFieldLabel(
    label: label,
    child: TextField(
      key: ValueKey('auth-$key'),
      controller: controls[key],
      focusNode: focuses[key],
      enabled: !busy,
      obscureText: secret && !(confirm ? confirmationVisible : visible),
      keyboardType: key == 'phone'
          ? TextInputType.phone
          : key == 'code'
          ? TextInputType.number
          : null,
      autocorrect: false,
      enableSuggestions: false,
      inputFormatters: key == 'phone'
          ? [
              FilteringTextInputFormatter.digitsOnly,
              LengthLimitingTextInputFormatter(11),
            ]
          : key == 'code'
          ? [
              FilteringTextInputFormatter.digitsOnly,
              LengthLimitingTextInputFormatter(6),
            ]
          : null,
      textInputAction:
          key == 'confirmation' ||
              smsMode && key == 'code' ||
              !verifying && key == 'password'
          ? TextInputAction.done
          : TextInputAction.next,
      onSubmitted: (_) =>
          key == 'confirmation' || !verifying && key == 'password'
          ? submit()
          : FocusScope.of(context).nextFocus(),
      autofillHints: key == 'phone'
          ? [AutofillHints.telephoneNumberNational]
          : key == 'code'
          ? [AutofillHints.oneTimeCode]
          : key == 'login_name'
          ? [AutofillHints.username]
          : null,
      onChanged: (_) => setState(() {
        errors.remove(key);
        if (key == 'phone') {
          controls['code']!.clear();
          dev = '';
        }
      }),
      decoration: InputDecoration(
        errorText: errors[key],
        errorMaxLines: 4,
        contentPadding: const EdgeInsets.symmetric(
          horizontal: 14,
          vertical: 16,
        ),
        suffixIcon: secret
            ? IconButton(
                tooltip: (confirm ? confirmationVisible : visible)
                    ? text('隐藏$label', 'Hide $label')
                    : text('显示$label', 'Show $label'),
                onPressed: () => setState(() {
                  if (confirm) {
                    confirmationVisible = !confirmationVisible;
                  } else {
                    visible = !visible;
                  }
                }),
                icon: Icon(
                  (confirm ? confirmationVisible : visible)
                      ? Icons.visibility_off_outlined
                      : Icons.visibility_outlined,
                ),
              )
            : null,
      ),
    ),
  );
  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    final title = widget.binding
        ? text('绑定手机号', 'Link mobile number')
        : smsMode
        ? text('验证码登录', 'SMS sign-in')
        : setup
        ? text('原短信账号设置密码', 'Set password for SMS account')
        : legacy
        ? (registerMode
              ? text('公司账号注册', 'Company account registration')
              : text('公司账号登录', 'Company account sign-in'))
        : widget.visitor
        ? (registerMode
              ? text('访客注册', 'Visitor registration')
              : text('访客登录', 'Visitor sign-in'))
        : registerMode
        ? text('手机号注册', 'Register with mobile')
        : text('手机号登录', 'Mobile sign-in');
    return Theme(
      data: merchantTheme(context),
      child: Scaffold(
        appBar: appBar(context, title),
        body: SafeArea(
          top: false,
          child: SingleChildScrollView(
            keyboardDismissBehavior: ScrollViewKeyboardDismissBehavior.onDrag,
            padding: const EdgeInsets.all(16),
            child: Align(
              alignment: Alignment.topCenter,
              child: ConstrainedBox(
                constraints: const BoxConstraints(maxWidth: 440),
                child: Card(
                  margin: EdgeInsets.zero,
                  child: Padding(
                    padding: const EdgeInsets.all(20),
                    child: AutofillGroup(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.stretch,
                        children: [
                          Text(
                            widget.binding
                                ? text(
                                    '验证后可用手机号登录，原账号与资料保留。可稍后在账号设置中完成。',
                                    'Verify your mobile number to sign in. Your account and data stay together. You may do this later in account settings.',
                                  )
                                : setup
                                ? text(
                                    '仅用于尚未设置密码的原短信账号。',
                                    'For original SMS accounts without a password.',
                                  )
                                : widget.visitor
                                ? text(
                                    '收藏优质厂商与产品，跨设备同步。',
                                    'Save manufacturers and products, and sync across devices.',
                                  )
                                : registerMode
                                ? text(
                                    legacy
                                        ? '创建公司账号 → 完善资料 → 预览官网 → 提交审核'
                                        : '验证手机号 → 完善企业资料 → 提交审核',
                                    legacy
                                        ? 'Create account → Complete profile → Preview website → Submit'
                                        : 'Verify mobile → Complete profile → Submit for review',
                                  )
                                : text(
                                    '登录后管理企业展厅与产品。',
                                    'Manage your company showroom and products.',
                                  ),
                            style: const TextStyle(
                              color: Color(0xff637b95),
                              height: 1.5,
                            ),
                          ),
                          space(20),
                          input(
                            legacy ? 'login_name' : 'phone',
                            legacy
                                ? text('公司账号', 'Company account')
                                : text('手机号', 'Mobile number'),
                          ),
                          if (verifying) ...[
                            space(16),
                            Row(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                Expanded(
                                  child: input(
                                    'code',
                                    text('短信验证码', 'SMS code'),
                                  ),
                                ),
                                const SizedBox(width: 8),
                                ConstrainedBox(
                                  constraints: const BoxConstraints(
                                    minHeight: 56,
                                    maxWidth: 124,
                                  ),
                                  child: OutlinedButton(
                                    style: OutlinedButton.styleFrom(
                                      padding: const EdgeInsets.symmetric(
                                        horizontal: 10,
                                        vertical: 14,
                                      ),
                                    ),
                                    onPressed: busy || cooldown > 0
                                        ? null
                                        : () => submit(send: true),
                                    child: Text(
                                      cooldown > 0
                                          ? '${cooldown}s'
                                          : text('发送验证码', 'Send code'),
                                      textAlign: TextAlign.center,
                                    ),
                                  ),
                                ),
                              ],
                            ),
                            if (dev.isNotEmpty) Text('${m.t('devCode')}: $dev'),
                          ],
                          if (smsMode)
                            Text(
                              text(
                                '仅登录已有且验证过手机号的业务账号。新用户请先注册，不会自动创建厂商。',
                                'Existing verified accounts only. Register separately; sign-in never creates a supplier.',
                              ),
                            ),
                          if (!widget.binding && !smsMode) ...[
                            space(16),
                            input(
                              'password',
                              (registerMode || setup)
                                  ? text('设置密码', 'Set password')
                                  : text('密码', 'Password'),
                              secret: true,
                            ),
                            if (registerMode || setup) ...[
                              space(16),
                              input(
                                'confirmation',
                                text('确认密码', 'Confirm password'),
                                secret: true,
                                confirm: true,
                              ),
                              space(12),
                              Text(
                                text(
                                  '密码长度为8–20位，建议使用字母和数字组合。',
                                  'Use 8–20 characters; letters and numbers are recommended.',
                                ),
                                style: const TextStyle(
                                  fontSize: 12,
                                  color: Color(0xff637b95),
                                ),
                              ),
                            ],
                          ],
                          if (registerMode)
                            NativeRegistrationAgreements(
                              key: ValueKey(
                                'registration-agreements-$agreementGeneration',
                              ),
                              onChanged: (refs) => acceptedAgreements = refs,
                            ),
                          if (error.isNotEmpty)
                            Padding(
                              padding: const EdgeInsets.only(top: 12),
                              child: Text(
                                error,
                                style: const TextStyle(color: Colors.red),
                              ),
                            ),
                          space(20),
                          FilledButton(
                            style: FilledButton.styleFrom(
                              minimumSize: const Size(0, 48),
                            ),
                            onPressed: busy || unconfirmed
                                ? null
                                : () => submit(),
                            child: Text(
                              busy
                                  ? m.t('loading')
                                  : widget.binding
                                  ? text('验证并绑定', 'Verify and link')
                                  : setup
                                  ? text('设置密码并登录', 'Set password and sign in')
                                  : registerMode
                                  ? text('注册并继续', 'Register and continue')
                                  : text('登录', 'Sign in'),
                            ),
                          ),
                          if (!widget.binding &&
                              !registerMode &&
                              !setup &&
                              !legacy)
                            TextButton(
                              onPressed: busy
                                  ? null
                                  : () {
                                      final next = !smsMode;
                                      mode();
                                      setState(() => smsMode = next);
                                    },
                              child: Text(
                                smsMode
                                    ? text('使用密码登录', 'Use password')
                                    : text(
                                        '短信验证码登录',
                                        'SMS verification sign-in',
                                      ),
                              ),
                            ),
                          if (widget.binding)
                            TextButton(
                              onPressed: busy
                                  ? null
                                  : () => Navigator.pop(context, false),
                              child: Text(text('暂时跳过', 'Skip for now')),
                            )
                          else ...[
                            space(8),
                            TextButton(
                              onPressed: busy
                                  ? null
                                  : () => mode(
                                      registration: !registerMode && !setup,
                                      old:
                                          widget.companyAccount &&
                                          !widget.visitor,
                                    ),
                              child: Text(
                                unconfirmed
                                    ? text('已有账号？登录继续', 'Sign in to continue')
                                    : registerMode
                                    ? text(
                                        '已有账号？登录',
                                        'Already registered? Sign in',
                                      )
                                    : text(
                                        '注册${widget.visitor ? '访客' : '厂商'}账号',
                                        widget.visitor
                                            ? 'Register visitor account'
                                            : 'Register manufacturer account',
                                      ),
                              ),
                            ),
                            if (!registerMode && !setup && !legacy)
                              TextButton(
                                onPressed: busy ? null : () => mode(old: true),
                                child: Text(
                                  text('原账号登录', 'Original account sign-in'),
                                ),
                              ),
                            if (legacy || setup)
                              TextButton(
                                onPressed: busy ? null : () => mode(),
                                child: Text(
                                  text('手机号密码登录', 'Mobile password sign-in'),
                                ),
                              ),
                            if (!registerMode)
                              ExpansionTile(
                                tilePadding: EdgeInsets.zero,
                                title: Text(
                                  text('登录帮助', 'Sign-in help'),
                                  style: const TextStyle(fontSize: 13),
                                ),
                                children: [
                                  Text(
                                    text(
                                      '忘记密码或手机号冲突请联系平台管理员。已有展厅请使用原账号登录后绑定手机号。',
                                      'Contact the platform administrator for forgotten passwords or mobile conflicts. For an existing showroom, sign in with the original account and link a mobile number.',
                                    ),
                                  ),
                                  TextButton(
                                    onPressed: busy
                                        ? null
                                        : () => mode(setting: true),
                                    child: Text(
                                      text(
                                        '原短信账号设置密码',
                                        'Set password for original SMS account',
                                      ),
                                    ),
                                  ),
                                ],
                              ),
                          ],
                        ],
                      ),
                    ),
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

Future<bool> confirmMerchantUpgrade(BuildContext context) async {
  final m = ModelScope.of(context);
  final ok = await showDialog<bool>(
    context: context,
    builder: (c) => AlertDialog(
      title: Text(syncText(c, '申请免费企业展厅', 'Upgrade to manufacturer')),
      content: Text(
        syncText(
          c,
          '将使用当前访客账号继续入驻，保留账号、收藏和已有资料。完善企业资料并审核通过后才会公开展厅。',
          'Continue with this visitor account and keep your favorites and data. Your showroom becomes public only after the company profile is reviewed.',
        ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.pop(c, false),
          child: Text(m.t('cancel')),
        ),
        FilledButton(
          onPressed: () => Navigator.pop(c, true),
          child: Text(syncText(c, '确认申请', 'Confirm upgrade')),
        ),
      ],
    ),
  );
  if (ok != true || !context.mounted) return false;
  final source = m.baseUrl,
      result = asMap(
        await m.api(
          '/auth/me/merchant-upgrade',
          method: 'POST',
          data: {'confirmed': true},
        ),
      );
  if (!context.mounted || source != m.baseUrl) return false;
  await m.signIn(result);
  return true;
}

Future<void> authenticateManufacturerEntry(
  BuildContext context, {
  bool register = true,
}) async {
  final m = ModelScope.of(context), source = m.baseUrl;
  if (m.token.isNotEmpty && m.account['role'] == 'visitor') {
    try {
      if ((await m.clientCapabilities())['phone_password_auth'] != true) {
        throw ApiFailure('phoneAuthUpgradeRequired');
      }
      if (!context.mounted || source != m.baseUrl) return;
      if (!await confirmMerchantUpgrade(context)) return;
      if (!context.mounted || source != m.baseUrl) return;
      m.newlyRegistered = true;
      await editMerchant(context, null, 'vendor');
    } catch (e) {
      if (context.mounted) showError(context, e);
    }
    return;
  }
  bool registered = false;
  final ok = await Navigator.push<bool>(
    context,
    MaterialPageRoute(
      builder: (_) => LoginScreen(
        register: register,
        onRegistered: () => registered = true,
      ),
    ),
  );
  if (!context.mounted || ok != true || source != m.baseUrl || !isMerchant(m)) {
    return;
  }
  if (registered) {
    m.newlyRegistered = true;
    await editMerchant(context, null, 'vendor');
  } else {
    push(context, const MerchantScreen());
  }
}
