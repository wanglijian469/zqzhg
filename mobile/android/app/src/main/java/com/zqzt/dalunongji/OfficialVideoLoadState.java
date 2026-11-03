package com.zqzt.dalunongji;

/** One explicit open/retry is one generation. Slow loading is not a fatal error. */
public final class OfficialVideoLoadState {
 private int generation;
 private boolean active,ready,slow;
 public int begin(){generation++;active=true;ready=false;slow=false;return generation;}
 public boolean current(int attempt){return active&&attempt==generation;}
 public boolean slow(int attempt){if(!current(attempt)||ready||slow)return false;slow=true;return true;}
 public boolean ready(int attempt){if(!current(attempt)||ready)return false;ready=true;return true;}
 public void stop(){active=false;}
}
