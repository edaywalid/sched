import { motion, useMotionValue, useSpring, useTransform } from "motion/react";
import { useEffect, type ReactNode } from "react";

interface FadeInProps {
  children: ReactNode;
  delay?: number;
  className?: string;
}

export function FadeIn({ children, delay = 0, className }: FadeInProps) {
  return (
    <motion.div
      initial={{ opacity: 0, y: 4 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.24, delay, ease: "easeOut" }}
      className={className}
    >
      {children}
    </motion.div>
  );
}

interface StaggerProps {
  children: ReactNode;
  gap?: number;
  className?: string;
}

export function Stagger({ children, gap = 0.04, className }: StaggerProps) {
  return (
    <motion.div
      initial="hidden"
      animate="show"
      variants={{
        hidden: {},
        show: { transition: { staggerChildren: gap } },
      }}
      className={className}
    >
      {children}
    </motion.div>
  );
}

export function StaggerItem({
  children,
  className,
}: {
  children: ReactNode;
  className?: string;
}) {
  return (
    <motion.div
      variants={{
        hidden: { opacity: 0, y: 6 },
        show: { opacity: 1, y: 0, transition: { duration: 0.2 } },
      }}
      className={className}
    >
      {children}
    </motion.div>
  );
}

interface CounterProps {
  value: number;
  format?: (n: number) => string;
  className?: string;
}

export function Counter({
  value,
  format = (n) => n.toLocaleString(),
  className,
}: CounterProps) {
  const motionValue = useMotionValue(0);
  const spring = useSpring(motionValue, { stiffness: 80, damping: 18, mass: 0.5 });
  const display = useTransform(spring, (latest) => format(Math.round(latest)));

  useEffect(() => {
    motionValue.set(value);
  }, [motionValue, value]);

  return <motion.span className={className}>{display}</motion.span>;
}
