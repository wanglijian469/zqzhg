package com.zqzt.dalunongji;

/** Optional diagnostics never turn normal playback into a permanent help/menu area. */
public final class OfficialVideoUiState {
 private String phase="loading";
 public void event(String event) {
  if("loading".equals(event)||"loaded".equals(event)||"error".equals(event))phase=event;
  else if("slow".equals(event)&&"loading".equals(phase))phase=event;
 }
 public boolean busy(){return "loading".equals(phase);}
 public boolean statusVisible(){return !"loaded".equals(phase);}
 public boolean retryVisible(){return "error".equals(phase)||"slow".equals(phase);}
 public boolean canRetry(){return !busy();}
}
