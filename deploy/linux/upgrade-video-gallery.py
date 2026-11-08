#!/usr/bin/env python3
"""Upgrade an existing zqzt installation to the video gallery release."""
import json
from urllib.parse import parse_qs, urlsplit

import importlib.util
from pathlib import Path
import sys
from urllib.error import HTTPError

sys.dont_write_bytecode = True
spec = importlib.util.spec_from_file_location('showroom_seo', Path(__file__).with_name('upgrade-showroom-seo.py'))
seo = importlib.util.module_from_spec(spec)
spec.loader.exec_module(seo)
seo.ops.VERSION = '1.10.11'
seo.ops.STATE = seo.ops.BASE / 'shared/upgrade-v1.10.11-private.json'
check_seo_candidate = seo.candidate_check
check_seo_production = seo.production_check


def verify_videos(fetch):
    def read(path):
        status, body = fetch('/api/v1' + path)
        assert status == 200, 'video_api_http_error_' + path
        return json.loads(body)

    config = read('/public/config')['capabilities']
    assert config['video_api_version'] == 3 and config['douyin_video'], 'video_capability_unavailable'
    path = '/public/showrooms/v-64/resources?kind=video&video_api_version='
    old = read(path + '2')['items']
    current = read(path + '3')['items']
    expected = [row for row in current if row['data'].get('video_source') == 'DOUYIN' and row['data']['purpose'] != 'other']
    assert {row['id'] for row in old if row['data'].get('video_source') == 'DOUYIN'} == {row['id'] for row in expected}, 'legacy_video_visibility_changed'
    assert expected, 'published_chungeng_douyin_missing'
    for row in expected:
        check = read('/public/resources/' + str(row['id']) + '/playback?video_api_version=3')
        assert check['status'] == 'READY' and check['canonical_url'] == row['data']['canonical_video_url'], 'published_douyin_playback_unavailable'
        player = urlsplit(check['player_url'])
        assert (player.scheme, player.netloc, player.path) == ('https', 'open.douyin.com', '/player/video')
        assert parse_qs(player.query) == {'vid': [check['video_id']], 'autoplay': ['0']}
    return {'videoAPIVersion': 3, 'legacyListVerified': True, 'chungengOfficialPlayersReady': len(expected)}


def candidate_check(port, release):
    def fetch(path):
        try:
            return seo.ops.get(path, port)
        except HTTPError as error:
            return error.code, error.read()
    return dict(check_seo_candidate(port, release), **verify_videos(fetch))


def production_check(release):
    return dict(check_seo_production(release), **verify_videos(seo.production_fetch))


seo.candidate_check = candidate_check
seo.production_check = production_check

if __name__ == '__main__':
    try:
        seo.main()
    except Exception as error:
        print(json.dumps({'failed': type(error).__name__, 'reason': str(error)}))
        sys.exit(1)
