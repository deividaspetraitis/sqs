from locust import HttpUser, task
from pairs import *

class Routes(HttpUser):
    # on_start is called when a Locust start before any task is scheduled.
    def on_start(self):
        pass

    # on_stop is called when the TaskSet is stopping
    def on_stop(self):
        pass

    @task
    def routesUOSMOUSDC(self):
        self.client.get(f"/router/routes?tokenOut={UOSMO}&tokenInDenom={USDC}")

    @task
    def routesUSDCUOSMO(self):
        self.client.get(f"/router/routes?tokenOut={USDC}&tokenInDenom={UOSMO}")

