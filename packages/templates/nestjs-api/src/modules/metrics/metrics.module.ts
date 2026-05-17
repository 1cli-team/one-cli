import { Module } from "@nestjs/common";
import {
  makeCounterProvider,
  makeHistogramProvider,
} from "@willsoto/nestjs-prometheus";

const httpRequestsCounter = makeCounterProvider({
  name: "http_requests_total",
  help: "Total number of HTTP requests",
  labelNames: ["method", "route", "status"],
});

const httpRequestDurationHistogram = makeHistogramProvider({
  name: "http_request_duration_seconds",
  help: "Duration of HTTP requests in seconds",
  labelNames: ["method", "route", "status"],
  buckets: [0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10],
});

const appInfoCounter = makeCounterProvider({
  name: "app_info",
  help: "Application information",
  labelNames: ["version", "environment"],
});

@Module({
  providers: [
    httpRequestsCounter,
    httpRequestDurationHistogram,
    appInfoCounter,
  ],
  exports: [httpRequestsCounter, httpRequestDurationHistogram, appInfoCounter],
})
export class MetricsModule {}
