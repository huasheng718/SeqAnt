import React from "react";
import primary from "../../web/images/seqant/logo-primary.svg";
import white from "../../web/images/seqant/logo-white.svg";
import "./home-logo.css";

export default function HomeLogo({ className = "home-logo" }) {
  return (
    <span className={className} role="img" aria-label="SeqAnt">
      <img className="seqant-logo-light" src={primary} alt="" width="1020" height="242" />
      <img className="seqant-logo-dark" src={white} alt="" width="1020" height="242" />
    </span>
  );
}
