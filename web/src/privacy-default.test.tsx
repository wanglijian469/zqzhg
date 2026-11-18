// @vitest-environment jsdom
import {afterEach,expect,it,vi} from 'vitest';
import {analyticsAllowed,setAnalyticsConsent} from './privacy';
afterEach(()=>{vi.unstubAllGlobals();localStorage.clear()});
it('defaults on only without a saved choice and preserves explicit opt-out',()=>{localStorage.clear();expect(analyticsAllowed()).toBe(true);setAnalyticsConsent(false);expect(analyticsAllowed()).toBe(false);expect(localStorage.getItem('zqzt.privacy.analytics')).toBe('no');setAnalyticsConsent(true);expect(analyticsAllowed()).toBe(true)});
it('withdrawal removes pending analytics and identifiers but keeps the off preference',()=>{localStorage.setItem('zqzt.analytics.queue','private-queue');localStorage.setItem('zqzt.analytics.visitor','id');setAnalyticsConsent(false);expect(localStorage.getItem('zqzt.analytics.queue')).toBeNull();expect(localStorage.getItem('zqzt.analytics.visitor')).toBeNull();expect(analyticsAllowed()).toBe(false)});
