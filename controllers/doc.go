package controllers

/*
+-------------+                     +---------+   +---------+  +-----------+  +-------------+
| Reconciler  |                     | Planner |   | Sourcer |  | Executor  |  | Terraformer |
+-------------+                     +---------+   +---------+  +-----------+  +-------------+

------>| NextPlan(Environment) Step
       |-------------------------------->|
       |                                 |
       |                                 | Fetch()
       |                                 |------------->|
       |                                 |              |
       | Accept(Step)
       |------------------------------------------------------------>|
                                                                     |
                                                                     | Init()
                                                                     |-------------->|
                                                                     |               |
                                                          Info(Step) |               |
       |<------------------------------------------------------------|               |
<------|                                                             |               |
                                                        Update(Step) |               |
       |<------------------------------------------------------------|               |
       |                                                             |               |
       | Update(Step,Environment)
       |-------------------------------->|
<------|                                 |

*/

/*
Created with https://textart.io/sequence
object Reconciler Planner Sourcer Executor Terraformer
Reconciler->Planner: NextPlan(Environment) Step
Planner->Sourcer: Fetch()
Reconciler->Executor: Accept(Step)
Executor->Terraformer: Init()
Executor->Reconciler: Info(Step)
Executor->Reconciler: Update(Step)
*/
