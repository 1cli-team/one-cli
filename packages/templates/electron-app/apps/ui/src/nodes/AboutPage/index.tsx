import React, { FC } from "react";

const AboutPage: FC = () => {
  return (
    <section className="space-y-3">
      <h2 className="text-2xl font-semibold">About</h2>
      <p className="text-muted-foreground">
        Minimal electron + react template. Styled with tailwind v4 tokens.
      </p>
    </section>
  );
};

export default AboutPage;
