import React from "react";
import HomeLogo from "./HomeLogo";
import "./home-welcome.css";

export default function HomeWelcome() {
  return (
    <div className="welcome home-welcome">
      <h1 className="home-welcome-title"><HomeLogo /></h1>
      <p className="home-welcome-subtitle">让智能体有序协作，让复杂任务自主完成。</p>
    </div>
  );
}
